package storage

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/vars"
)

const timelogFileExt = ".log"

// FileTimelogRepository persists timelogs as todo.txt-format files
// under <dataDir>/<DefaultTimelogDirName>/<YYYY-MM-DD>.log. Layout
// matches the legacy on-disk format byte-for-byte so existing data
// migrates without conversion.
//
// Save rewrites the whole file, rather than the legacy append-only
// approach. Whole-file rewrite lets the persistent Timelog.Append
// method use a simple Save call without tracking which entries are new
// since the last save. Like every write, it goes through
// transaction.go: locked against other htt processes, checked against
// other programs, and atomic, the prior version kept as .bak.
type FileTimelogRepository struct {
	dataDir string
	hooks   hooks // seams for storage's own tests
}

func NewFileTimelogRepository(dataDir string) *FileTimelogRepository {
	return &FileTimelogRepository{dataDir: dataDir}
}

func (r *FileTimelogRepository) logDir() string {
	return filepath.Join(r.dataDir, vars.DefaultTimelogDirName)
}

// dirs are the directories r writes, which a transaction of r's locks:
// the log directory.
func (r *FileTimelogRepository) dirs() []string {
	return []string{r.logDir()}
}

// LogPath is the file that holds date's timelog.
func (r *FileTimelogRepository) LogPath(date time.Time) string {
	return filepath.Join(r.logDir(), date.Format("2006-01-02")+timelogFileExt)
}

func (r *FileTimelogRepository) CurrentLogPath() string {
	return r.LogPath(time.Now())
}

func (r *FileTimelogRepository) Today() (*domain.Timelog, error) {
	return r.Day(time.Now())
}

func (r *FileTimelogRepository) Day(date time.Time) (*domain.Timelog, error) {
	file, err := readFile(r.LogPath(date))
	if err != nil {
		return nil, err
	}
	return r.parseLog(r, date, file.data)
}

// parseLog makes date's timelog of its file's content, data (nil for a
// file that doesn't exist yet), wired to repo: r, or a transaction of
// r's.
func (r *FileTimelogRepository) parseLog(repo domain.TimelogRepository, date time.Time, data []byte) (*domain.Timelog, error) {
	l := domain.NewTimelog(repo, date)
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
			return nil, fmt.Errorf("parse %s:%d: %w", r.LogPath(date), lineNo, err)
		}
		// Line is the entry's index in l.Entries, as for todo tasks,
		// not its file line; FileLine is the file line, for messages
		// about the file.
		task.Line = len(l.Entries)
		task.FileLine = lineNo + 1
		l.Entries = append(l.Entries, task)
		lineNo++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", r.LogPath(date), err)
	}
	return l, nil
}

// Save writes l's file as a transaction of its own. Timelog.Append and
// AppendAt save within their own transactions (Transact).
func (r *FileTimelogRepository) Save(l *domain.Timelog) error {
	return r.Transact(func(tx domain.TimelogRepository) error { return tx.Save(l) })
}

// Transact runs change as one transaction
// (domain.TimelogRepository.Transact) over the log directory, through
// transaction.go. change gets a fileTimelogTx.
func (r *FileTimelogRepository) Transact(change func(tx domain.TimelogRepository) error) error {
	return transact(r.dirs(), r.hooks, func(t *transaction) error {
		tx := &fileTimelogTx{r: r, t: t}
		defer func() { tx.over = true }()
		return change(tx)
	})
}

// fileTimelogTx is a FileTimelogRepository within one run of a
// transaction's change, as fileTx is a FileRepository.
type fileTimelogTx struct {
	r    *FileTimelogRepository
	t    *transaction
	over bool
}

var _ domain.TimelogRepository = (*fileTimelogTx)(nil)

func (tx *fileTimelogTx) Today() (*domain.Timelog, error) { return tx.Day(time.Now()) }

func (tx *fileTimelogTx) Day(date time.Time) (*domain.Timelog, error) {
	data, _, err := tx.t.Read(tx.r.LogPath(date))
	if err != nil {
		return nil, err
	}
	return tx.r.parseLog(tx, date, data)
}

func (tx *fileTimelogTx) Save(l *domain.Timelog) error {
	if l == nil {
		return errors.New("nil timelog")
	}
	if tx.over {
		return errOver
	}
	tx.t.Write(tx.r.LogPath(l.Date), logFile(l), true)
	return nil
}

func (tx *fileTimelogTx) CurrentLogPath() string { return tx.r.CurrentLogPath() }

func (tx *fileTimelogTx) LogPath(date time.Time) string { return tx.r.LogPath(date) }

// Transact runs change as part of this transaction, while its change
// runs; afterwards, as a transaction of its own.
func (tx *fileTimelogTx) Transact(change func(tx domain.TimelogRepository) error) error {
	if tx.over {
		return tx.r.Transact(change)
	}
	return change(tx)
}

// logFile is l's file: an entry a line, each entry's Raw. Each entry's
// FileLine becomes its line there, for messages about it.
func logFile(l *domain.Timelog) []byte {
	var b bytes.Buffer
	for i, entry := range l.Entries {
		b.WriteString(entry.Raw)
		b.WriteByte('\n')
		entry.FileLine = i + 1
	}
	return b.Bytes()
}
