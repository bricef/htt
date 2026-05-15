package storage

import (
	"errors"
	"time"

	"github.com/bricef/htt/internal/domain"
)

// MemoryTimelogRepository is an in-memory domain.TimelogRepository for
// tests. Not safe for concurrent use; tests don't need it.
//
// Entries are keyed by date string (YYYY-MM-DD), so two times within
// the same calendar day map to the same Timelog — matching the
// file-backed layout where each day has a single .log file.
type MemoryTimelogRepository struct {
	entries map[string][]*domain.Task
}

func NewMemoryTimelogRepository() *MemoryTimelogRepository {
	return &MemoryTimelogRepository{entries: map[string][]*domain.Task{}}
}

func (r *MemoryTimelogRepository) Today() (*domain.Timelog, error) {
	return r.Day(time.Now())
}

func (r *MemoryTimelogRepository) Day(date time.Time) (*domain.Timelog, error) {
	l := domain.NewTimelog(r, date)
	stored := r.entries[timelogKey(date)]
	l.Entries = make([]*domain.Task, len(stored))
	copy(l.Entries, stored)
	return l, nil
}

func (r *MemoryTimelogRepository) Save(l *domain.Timelog) error {
	if l == nil {
		return errors.New("nil timelog")
	}
	cp := make([]*domain.Task, len(l.Entries))
	copy(cp, l.Entries)
	r.entries[timelogKey(l.Date)] = cp
	return nil
}

// CurrentLogPath returns the empty string for the in-memory repo;
// the path concept is meaningless without a filesystem. The CLI's
// `htt log edit` command treats an empty path as "not supported by
// this repo". Production runs with FileTimelogRepository where the
// path is real.
func (r *MemoryTimelogRepository) CurrentLogPath() string {
	return ""
}

// timelogKey maps a time.Time to its YYYY-MM-DD calendar-day string.
// The file impl uses the same format for its on-disk filename.
func timelogKey(t time.Time) string {
	return t.Format("2006-01-02")
}
