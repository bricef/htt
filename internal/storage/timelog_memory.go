package storage

import (
	"errors"
	"fmt"
	"time"

	"github.com/bricef/htt/internal/domain"
)

// MemoryTimelogRepository is an in-memory domain.TimelogRepository for
// tests. Not safe for concurrent use; tests don't need it.
//
// Entries are keyed by date string (YYYY-MM-DD), so two times within
// the same calendar day map to the same Timelog — matching the
// file-backed layout where each day has a single .log file.
//
// Save and Day deep-copy each Task (re-parsed from its Raw) rather
// than storing/returning shared pointers. This matches the
// file-backed repo's serialize-then-reparse semantics: mutations to a
// Task after Save cannot leak into stored state, and mutations to
// entries returned by Day cannot leak back into the next load.
// Without the deep copy, CLI display steps that mutate (e.g. the
// RemoveAnnotation that strips ts: for the "Logging entry:" line)
// silently corrupt later observations.
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
	entries, err := cloneTasks(stored)
	if err != nil {
		return nil, fmt.Errorf("clone entries for %s: %w", timelogKey(date), err)
	}
	l.Entries = entries
	return l, nil
}

func (r *MemoryTimelogRepository) Save(l *domain.Timelog) error {
	if l == nil {
		return errors.New("nil timelog")
	}
	cp, err := cloneTasks(l.Entries)
	if err != nil {
		return fmt.Errorf("clone entries for save: %w", err)
	}
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

// cloneTasks returns a deep copy of tasks: each element is re-parsed
// via domain.NewTask from its Raw. This is how the file impl
// necessarily round-trips (write -> read), and the memory impl
// matches it so test observations do not see shared-pointer
// mutations.
func cloneTasks(tasks []*domain.Task) ([]*domain.Task, error) {
	out := make([]*domain.Task, len(tasks))
	for i, t := range tasks {
		clone, err := domain.NewTask(t.Raw)
		if err != nil {
			return nil, fmt.Errorf("re-parse entry %d (%q): %w", i, t.Raw, err)
		}
		out[i] = clone
	}
	return out, nil
}
