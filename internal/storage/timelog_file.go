package storage

import (
	"bufio"
	"errors"
	"fmt"
	"os"
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
// Save rewrites the whole file (with a `.bak` rotation for crash
// recovery), rather than the legacy append-only approach. Whole-file
// rewrite lets the persistent Timelog.Append method use a simple
// Save call without tracking which entries are new since the last
// save. The `.bak` slot holds the prior version, mirroring the
// Context save pattern.
type FileTimelogRepository struct {
	dataDir string
}

func NewFileTimelogRepository(dataDir string) *FileTimelogRepository {
	return &FileTimelogRepository{dataDir: dataDir}
}

func (r *FileTimelogRepository) logDir() string {
	return filepath.Join(r.dataDir, vars.DefaultTimelogDirName)
}

func (r *FileTimelogRepository) logPath(date time.Time) string {
	return filepath.Join(r.logDir(), date.Format("2006-01-02")+timelogFileExt)
}

func (r *FileTimelogRepository) CurrentLogPath() string {
	return r.logPath(time.Now())
}

func (r *FileTimelogRepository) Today() (*domain.Timelog, error) {
	return r.Day(time.Now())
}

func (r *FileTimelogRepository) Day(date time.Time) (*domain.Timelog, error) {
	l := domain.NewTimelog(r, date)

	f, err := os.Open(r.logPath(date))
	if err != nil {
		if os.IsNotExist(err) {
			return l, nil
		}
		return nil, fmt.Errorf("open %s: %w", r.logPath(date), err)
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
			return nil, fmt.Errorf("parse %s:%d: %w", r.logPath(date), lineNo, err)
		}
		task.Line = lineNo
		l.Entries = append(l.Entries, task)
		lineNo++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", r.logPath(date), err)
	}
	return l, nil
}

func (r *FileTimelogRepository) Save(l *domain.Timelog) error {
	if l == nil {
		return errors.New("nil timelog")
	}

	if err := os.MkdirAll(r.logDir(), dirMode); err != nil {
		return fmt.Errorf("ensure timelog dir: %w", err)
	}

	path := r.logPath(l.Date)
	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, path+".bak"); err != nil {
			return fmt.Errorf("backup existing timelog: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat existing timelog: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	for _, entry := range l.Entries {
		if _, err := fmt.Fprintln(f, entry.Raw); err != nil {
			return fmt.Errorf("write entry: %w", err)
		}
	}
	return nil
}
