package storage

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/bricef/htt/internal/domain"
)

const (
	fileExtension      = ".txt"
	currentContextFile = "current-context"
	dirMode            = 0700
	fileMode           = 0644
)

// FileRepository persists contexts as todo.txt-format files. Context
// files (per-name *.txt) live under dataDir; the current-context
// pointer file (currentContextFile) lives under pointerDir. The legacy
// layout kept the pointer under tracker_path while the per-context
// files lived under data_path — most installs map both to the same
// directory but users that overrode tracker_path independently relied
// on the split. Layout matches the legacy on-disk layout byte-for-byte
// so existing data migrates cleanly.
//
// Unlike the legacy todo.Context.Read/Sync pair, Context is read-only:
// it never writes the file back. This fixes the long-standing bug
// where todo.Context.Read called Add (which called Sync) on every
// line, causing every read to overwrite the file.
//
// Every write goes through transaction.go: locked against other htt
// processes, checked against other programs, and atomic, the previous
// version kept as .bak.
//
// Context names are sanitized at the path boundary via
// domain.ContextFileName (non-word characters become underscores) so
// `Context("../foo")` and `Save(&Context{Name: "../escape"})` can't
// write outside dataDir. SetCurrent already sanitized; the rest of
// the repo now matches it.
type FileRepository struct {
	dataDir    string
	pointerDir string
	hooks      hooks // seams for storage's own tests
}

// NewFileRepository constructs a repo with separate directories for
// context files and the current-context pointer. Most callers pass the
// same path for both; pointerDir is honoured separately so users with a
// custom tracker_path different from data_path keep their context
// selection across the storage rewrite.
func NewFileRepository(dataDir, pointerDir string) *FileRepository {
	return &FileRepository{dataDir: dataDir, pointerDir: pointerDir}
}

// ContextPath builds the on-disk path for the named context file
// under dataDir. Pure path builder; performs no I/O. Sanitizes the
// name the same way SetCurrent does so the path is identical for any
// caller — programmatic readers (Context / Save) and the $EDITOR
// shellout (`htt todo edit-done`, TUI EditFile) all agree on where
// the file lives.
func (r *FileRepository) ContextPath(name string) string {
	return filepath.Join(r.dataDir, domain.ContextFileName(name)+fileExtension)
}

// SameFile reports whether contexts a and b are kept in one file
// (domain.SameFiler): both files exist and the file system says they're
// the same, as for TODO.txt and todo.txt on a case-insensitive one, or
// a symlink. A context with no file yet can't be the same as another.
func (r *FileRepository) SameFile(a, b string) bool {
	fa, err := os.Stat(r.ContextPath(a))
	if err != nil {
		return false
	}
	fb, err := os.Stat(r.ContextPath(b))
	if err != nil {
		return false
	}
	return os.SameFile(fa, fb)
}

func (r *FileRepository) currentPointerPath() string {
	return filepath.Join(r.pointerDir, currentContextFile)
}

// dirs are the directories r writes, each of which a transaction of
// r's locks: the data directory and the pointer's.
func (r *FileRepository) dirs() []string {
	return []string{r.dataDir, r.pointerDir}
}

func (r *FileRepository) ContextNames() ([]string, error) {
	entries, err := os.ReadDir(r.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("list contexts: %w", err)
	}

	names := []string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, fileExtension) {
			continue
		}
		names = append(names, strings.TrimSuffix(name, fileExtension))
	}
	sort.Strings(names)
	return names, nil
}

func (r *FileRepository) Context(name string) (*domain.Context, error) {
	if name == "" {
		return nil, domain.ErrInvalidContextName
	}
	file, err := readFile(r.ContextPath(name))
	if err != nil {
		return nil, err
	}
	return r.parseContext(r, name, file.data)
}

// parseContext makes the named context of its file's content, data (nil
// for a file that doesn't exist yet), wired to repo: r, or a
// transaction of r's.
func (r *FileRepository) parseContext(repo domain.Repository, name string, data []byte) (*domain.Context, error) {
	ctx := domain.NewContext(repo, name)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineNo := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			lineNo++
			continue
		}
		task, err := domain.NewTask(line)
		if err != nil {
			return nil, fmt.Errorf("parse %s:%d: %w", r.ContextPath(name), lineNo, err)
		}
		// FileLine is the file line, for messages about the file (see
		// Task.DateWarnings). Line, the number commands like delete
		// take, is the task's index once arranged, below: blank lines
		// are skipped, and subtasks go under their parents.
		task.FileLine = lineNo + 1
		ctx.Tasks = append(ctx.Tasks, task)
		lineNo++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", r.ContextPath(name), err)
	}
	ctx.Arrange()
	return ctx, nil
}

// ContextLines returns the named context's lines as Context reads them,
// trimmed, blank lines left out, without parsing them
// (domain.LineSource). A context never saved has none.
func (r *FileRepository) ContextLines(name string) ([]string, error) {
	if name == "" {
		return nil, domain.ErrInvalidContextName
	}
	file, err := readFile(r.ContextPath(name))
	if err != nil {
		return nil, err
	}
	return r.lines(name, file.data)
}

// lines returns the lines of the named context's file content, data,
// trimmed, blank lines left out.
func (r *FileRepository) lines(name string, data []byte) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", r.ContextPath(name), err)
	}
	return lines, nil
}

func (r *FileRepository) Contexts() ([]*domain.Context, error) {
	names, err := r.ContextNames()
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Context, 0, len(names))
	for _, name := range names {
		ctx, err := r.Context(name)
		if err != nil {
			return nil, err
		}
		out = append(out, ctx)
	}
	return out, nil
}

// Save writes ctx's file as a transaction of its own: locked,
// atomically, the previous version kept as .bak. The domain's
// operations save within their own transactions (Transact).
func (r *FileRepository) Save(ctx *domain.Context) error {
	return r.Transact(func(tx domain.Repository) error { return tx.Save(ctx) })
}

// Transact runs change as one transaction (domain.Repository.Transact)
// over the context files and the current-context pointer, through
// transaction.go. change gets a fileTx.
func (r *FileRepository) Transact(change func(tx domain.Repository) error) error {
	return transact(r.dirs(), r.hooks, func(t *transaction) error {
		tx := &fileTx{r: r, t: t}
		defer func() { tx.over = true }()
		return change(tx)
	})
}

// fileTx is a FileRepository within one run of a transaction's change:
// it reads files through the transaction, which keeps what it read for
// the check, and saves into it, held until the change returns. Once the
// change has returned it saves nothing, and Transact on it starts a
// transaction of its own.
type fileTx struct {
	r    *FileRepository
	t    *transaction
	over bool
}

var (
	_ domain.Repository = (*fileTx)(nil)
	_ domain.LineSource = (*fileTx)(nil)
	_ domain.SameFiler  = (*fileTx)(nil)
)

// errOver is a save to a transaction's repository once its change has
// returned: a context it loaded, kept and changed afterwards.
var errOver = errors.New("save outside the transaction that loaded the context")

func (tx *fileTx) Context(name string) (*domain.Context, error) {
	if name == "" {
		return nil, domain.ErrInvalidContextName
	}
	data, _, err := tx.t.Read(tx.r.ContextPath(name))
	if err != nil {
		return nil, err
	}
	return tx.r.parseContext(tx, name, data)
}

func (tx *fileTx) ContextLines(name string) ([]string, error) {
	if name == "" {
		return nil, domain.ErrInvalidContextName
	}
	data, _, err := tx.t.Read(tx.r.ContextPath(name))
	if err != nil {
		return nil, err
	}
	return tx.r.lines(name, data)
}

// ContextNames lists the contexts on disk and those the change has
// saved, which aren't on disk yet.
func (tx *fileTx) ContextNames() ([]string, error) {
	names, err := tx.r.ContextNames()
	if err != nil {
		return nil, err
	}
	for _, path := range tx.t.Paths() {
		if filepath.Dir(path) == filepath.Clean(tx.r.dataDir) && strings.HasSuffix(path, fileExtension) {
			if name := strings.TrimSuffix(filepath.Base(path), fileExtension); !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

func (tx *fileTx) Contexts() ([]*domain.Context, error) {
	names, err := tx.ContextNames()
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Context, 0, len(names))
	for _, name := range names {
		ctx, err := tx.Context(name)
		if err != nil {
			return nil, err
		}
		out = append(out, ctx)
	}
	return out, nil
}

func (tx *fileTx) CurrentContextName() (string, error) {
	data, exists, err := tx.t.Read(tx.r.currentPointerPath())
	if err != nil {
		return "", err
	}
	if !exists {
		return domain.DefaultContextName, nil
	}
	return pointerName(data), nil
}

func (tx *fileTx) CurrentContext() (*domain.Context, error) {
	name, err := tx.CurrentContextName()
	if err != nil {
		return nil, err
	}
	return tx.Context(name)
}

func (tx *fileTx) SetCurrent(name string) error {
	sanitized := domain.ContextFileName(name)
	if sanitized == "" {
		return domain.ErrInvalidContextName
	}
	if tx.over {
		return errOver
	}
	tx.t.Write(tx.r.currentPointerPath(), []byte(sanitized), false)
	return nil
}

func (tx *fileTx) Save(ctx *domain.Context) error {
	if ctx == nil || ctx.Name == "" {
		return domain.ErrInvalidContextName
	}
	if tx.over {
		return errOver
	}
	tx.t.Write(tx.r.ContextPath(ctx.Name), contextFile(ctx), true)
	return nil
}

func (tx *fileTx) ContextPath(name string) string { return tx.r.ContextPath(name) }

func (tx *fileTx) SameFile(a, b string) bool { return tx.r.SameFile(a, b) }

// Transact runs change as part of this transaction, while its change
// runs; afterwards, as a transaction of its own.
func (tx *fileTx) Transact(change func(tx domain.Repository) error) error {
	if tx.over {
		return tx.r.Transact(change)
	}
	return change(tx)
}

// contextFile is ctx's file: a task a line, each task's Raw. Each
// task's FileLine becomes its line there, for messages about it.
func contextFile(ctx *domain.Context) []byte {
	var b bytes.Buffer
	for i, task := range ctx.Tasks {
		b.WriteString(task.Raw)
		b.WriteByte('\n')
		task.FileLine = i + 1
	}
	return b.Bytes()
}

// CurrentContextName reads the pointer as every read of htt's files is
// made (readFile): on Windows, opening it at the moment another htt
// renames a new one over it fails, and is tried again.
func (r *FileRepository) CurrentContextName() (string, error) {
	file, err := readFile(r.currentPointerPath())
	if err != nil {
		return "", err
	}
	if !file.exists {
		return domain.DefaultContextName, nil
	}
	return pointerName(file.data), nil
}

// pointerName is the context the current-context pointer's content, b,
// names: DefaultContextName when it's empty.
func pointerName(b []byte) string {
	name := strings.TrimSpace(string(b))
	if name == "" {
		return domain.DefaultContextName
	}
	return name
}

func (r *FileRepository) CurrentContext() (*domain.Context, error) {
	name, err := r.CurrentContextName()
	if err != nil {
		return nil, err
	}
	return r.Context(name)
}

// SetCurrent writes the current-context pointer as a transaction of its
// own.
func (r *FileRepository) SetCurrent(name string) error {
	if domain.ContextFileName(name) == "" {
		return domain.ErrInvalidContextName
	}
	return r.Transact(func(tx domain.Repository) error { return tx.SetCurrent(name) })
}
