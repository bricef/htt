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
//
// Its transactions (Transact) hold their saves until the change
// returns, and run it again when the store changed meanwhile, which
// Underneath lets a test do, as MemoryRepository's do.
type MemoryTimelogRepository struct {
	entries map[string][]*domain.Task
	changes int // the store's changes, for a transaction's check

	// Underneath, when set, runs in each transaction after its change
	// and before its check, as MemoryRepository.Underneath does.
	Underneath func(run int)
}

func NewMemoryTimelogRepository() *MemoryTimelogRepository {
	return &MemoryTimelogRepository{entries: map[string][]*domain.Task{}}
}

func (r *MemoryTimelogRepository) Today() (*domain.Timelog, error) {
	return r.Day(time.Now())
}

func (r *MemoryTimelogRepository) Day(date time.Time) (*domain.Timelog, error) {
	return r.load(r, date, r.entries[timelogKey(date)])
}

// load makes date's timelog of stored, copied, wired to repo: r, or a
// transaction of r's.
func (r *MemoryTimelogRepository) load(repo domain.TimelogRepository, date time.Time, stored []*domain.Task) (*domain.Timelog, error) {
	l := domain.NewTimelog(repo, date)
	entries, err := cloneTasks(stored)
	if err != nil {
		return nil, fmt.Errorf("clone entries for %s: %w", timelogKey(date), err)
	}
	l.Entries = entries
	return l, nil
}

// Save stores a copy of l's entries, at once: it changes the store
// under any transaction running, as another htt's save would.
func (r *MemoryTimelogRepository) Save(l *domain.Timelog) error {
	if l == nil {
		return errors.New("nil timelog")
	}
	cp, err := cloneTasks(l.Entries)
	if err != nil {
		return fmt.Errorf("clone entries for save: %w", err)
	}
	r.entries[timelogKey(l.Date)] = cp
	r.changes++
	return nil
}

// Transact runs change as one transaction
// (domain.TimelogRepository.Transact), as MemoryRepository.Transact
// does for contexts.
func (r *MemoryTimelogRepository) Transact(change func(tx domain.TimelogRepository) error) error {
	for run := 1; ; run++ {
		before := r.changes
		tx := &memoryTimelogTx{r: r, saved: map[string][]*domain.Task{}}
		err := change(tx)
		tx.over = true
		if err != nil {
			return err
		}
		if len(tx.order) == 0 {
			return nil
		}
		if r.Underneath != nil {
			r.Underneath(run)
		}
		if r.changes == before {
			for _, day := range tx.order {
				r.entries[day] = tx.saved[day]
			}
			r.changes++
			return nil
		}
		if run == runs {
			return fmt.Errorf("the memory store %w", domain.ErrKeepsChanging)
		}
	}
}

// memoryTimelogTx is a MemoryTimelogRepository within one run of a
// transaction's change, as memoryTx is a MemoryRepository.
type memoryTimelogTx struct {
	r     *MemoryTimelogRepository
	saved map[string][]*domain.Task
	order []string // the days saved, in order
	over  bool
}

var _ domain.TimelogRepository = (*memoryTimelogTx)(nil)

func (tx *memoryTimelogTx) Today() (*domain.Timelog, error) { return tx.Day(time.Now()) }

func (tx *memoryTimelogTx) Day(date time.Time) (*domain.Timelog, error) {
	stored, ok := tx.saved[timelogKey(date)]
	if !ok {
		stored = tx.r.entries[timelogKey(date)]
	}
	return tx.r.load(tx, date, stored)
}

func (tx *memoryTimelogTx) Save(l *domain.Timelog) error {
	if l == nil {
		return errors.New("nil timelog")
	}
	if tx.over {
		return errOver
	}
	cp, err := cloneTasks(l.Entries)
	if err != nil {
		return fmt.Errorf("clone entries for save: %w", err)
	}
	day := timelogKey(l.Date)
	if _, ok := tx.saved[day]; !ok {
		tx.order = append(tx.order, day)
	}
	tx.saved[day] = cp
	return nil
}

func (tx *memoryTimelogTx) CurrentLogPath() string { return "" }

func (tx *memoryTimelogTx) LogPath(time.Time) string { return "" }

// Transact runs change as part of this transaction, while its change
// runs; afterwards, as a transaction of its own.
func (tx *memoryTimelogTx) Transact(change func(tx domain.TimelogRepository) error) error {
	if tx.over {
		return tx.r.Transact(change)
	}
	return change(tx)
}

// CurrentLogPath returns the empty string for the in-memory repo;
// the path concept is meaningless without a filesystem. The CLI's
// `htt log edit` command treats an empty path as "not supported by
// this repo". Production runs with FileTimelogRepository where the
// path is real.
func (r *MemoryTimelogRepository) CurrentLogPath() string {
	return ""
}

// LogPath returns "" too, for the same reason.
func (r *MemoryTimelogRepository) LogPath(time.Time) string {
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
