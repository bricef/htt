package storage

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/utils"
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
// Context names are sanitized at the path boundary via
// utils.StringToFilename (non-word characters become underscores) so
// `Context("../foo")` and `Save(&Context{Name: "../escape"})` can't
// write outside dataDir. SetCurrent already sanitized; the rest of
// the repo now matches it.
type FileRepository struct {
	dataDir    string
	pointerDir string
}

// NewFileRepository constructs a repo with separate directories for
// context files and the current-context pointer. Most callers pass the
// same path for both; pointerDir is honoured separately so users with a
// custom tracker_path different from data_path keep their context
// selection across the storage rewrite.
func NewFileRepository(dataDir, pointerDir string) *FileRepository {
	return &FileRepository{dataDir: dataDir, pointerDir: pointerDir}
}

func (r *FileRepository) contextPath(name string) string {
	return filepath.Join(r.dataDir, utils.StringToFilename(name)+fileExtension)
}

func (r *FileRepository) currentPointerPath() string {
	return filepath.Join(r.pointerDir, currentContextFile)
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

	ctx := domain.NewContext(r, name)

	f, err := os.Open(r.contextPath(name))
	if err != nil {
		if os.IsNotExist(err) {
			return ctx, nil
		}
		return nil, fmt.Errorf("open %s: %w", r.contextPath(name), err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			lineNo++
			continue
		}
		task, err := domain.NewTask(line)
		if err != nil {
			return nil, fmt.Errorf("parse %s:%d: %w", r.contextPath(name), lineNo, err)
		}
		task.Line = lineNo
		ctx.Tasks = append(ctx.Tasks, task)
		lineNo++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", r.contextPath(name), err)
	}
	return ctx, nil
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

func (r *FileRepository) Save(ctx *domain.Context) error {
	if ctx == nil || ctx.Name == "" {
		return domain.ErrInvalidContextName
	}

	if err := os.MkdirAll(r.dataDir, dirMode); err != nil {
		return fmt.Errorf("ensure data dir: %w", err)
	}

	path := r.contextPath(ctx.Name)
	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, path+".bak"); err != nil {
			return fmt.Errorf("backup existing file: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat existing file: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	for _, task := range ctx.Tasks {
		if _, err := fmt.Fprintln(f, task.Raw); err != nil {
			return fmt.Errorf("write task: %w", err)
		}
	}
	return nil
}

func (r *FileRepository) CurrentContextName() (string, error) {
	b, err := os.ReadFile(r.currentPointerPath())
	if err != nil {
		if os.IsNotExist(err) {
			return domain.DefaultContextName, nil
		}
		return "", fmt.Errorf("read current-context: %w", err)
	}
	name := strings.TrimSpace(string(b))
	if name == "" {
		return domain.DefaultContextName, nil
	}
	return name, nil
}

func (r *FileRepository) CurrentContext() (*domain.Context, error) {
	name, err := r.CurrentContextName()
	if err != nil {
		return nil, err
	}
	return r.Context(name)
}

func (r *FileRepository) SetCurrent(name string) error {
	sanitized := utils.StringToFilename(name)
	if sanitized == "" {
		return domain.ErrInvalidContextName
	}
	if err := os.MkdirAll(r.dataDir, dirMode); err != nil {
		return fmt.Errorf("ensure data dir: %w", err)
	}
	if err := os.WriteFile(r.currentPointerPath(), []byte(sanitized), fileMode); err != nil {
		return fmt.Errorf("write current-context: %w", err)
	}
	return nil
}
