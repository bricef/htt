package domain

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// Timelog is one day's worth of activity entries. Each entry is a
// Task annotated with a "ts:<RFC3339>" timestamp by Append or AppendAt.
//
// The repo seam is the same pattern as Context: Timelogs handed out
// by a TimelogRepository carry a wired repo and can use Append and
// AppendAt (which save). Struct-literal Timelogs work for pure-method
// tests (IsEmpty, Timesheet) — those never touch the repo. Calling
// Append or AppendAt on a struct-literal Timelog will nil-deref; that's
// a programmer error, and we want it loud.
type Timelog struct {
	Date    time.Time
	Entries []*Task
	repo    TimelogRepository

	// changing is true while a transaction runs on the log (transact):
	// its repository is then the transaction's.
	changing bool
}

// NewTimelog returns a Timelog wired with the given repository.
// Intended for TimelogRepository implementations: external callers
// obtain Timelogs via TimelogRepository.Today or Day. Storage impls
// construct via this constructor and then populate Entries before
// returning the Timelog to a caller.
func NewTimelog(repo TimelogRepository, date time.Time) *Timelog {
	return &Timelog{
		Date:    date,
		Entries: []*Task{},
		repo:    repo,
	}
}

// IsEmpty reports whether the timelog has no entries.
func (l *Timelog) IsEmpty() bool {
	return len(l.Entries) == 0
}

// entryTimestamp pulls the ts: annotation off a timelog entry and
// parses it as RFC3339.
func entryTimestamp(t *Task) (time.Time, error) {
	raw, ok := t.Annotations[TimelogTimestampLabel]
	if !ok {
		return time.Time{}, fmt.Errorf("timelog entry missing %s annotation: %q", TimelogTimestampLabel, t.Raw)
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s=%q: %w", TimelogTimestampLabel, raw, err)
	}
	return parsed, nil
}

// Append annotates task with the current timestamp under
// TimelogTimestampLabel, appends it to the in-memory entries, and
// persists through the wired repo. The task pointer is mutated in
// place (Annotate mutates the receiver), so callers can read its
// post-annotation Raw / Annotations afterwards.
//
// The entry goes at the end, whatever the times of the entries before
// it; AppendAt places an entry by its time.
//
// Like AppendAt, it runs as one transaction of the log's repository
// (transact), on the day's log as the store holds it then.
func (l *Timelog) Append(task *Task) error {
	stamp := time.Now().UTC().Format(time.RFC3339)
	return l.transact(func() error {
		if _, err := task.Annotate(TimelogTimestampLabel, stamp); err != nil {
			return fmt.Errorf("annotate entry timestamp: %w", err)
		}
		l.Entries = append(l.Entries, task)
		return l.repo.Save(l)
	})
}

// transact runs body as one transaction of l's repository
// (TimelogRepository.Transact), on the day's log as the store holds it
// then, as Context's persistent methods run (Context.transact): l's
// entries become those the transaction reads, l's own entry objects
// when they're the same lines, and l's repository is the
// transaction's while body runs. body may run more than once.
func (l *Timelog) transact(body func() error) error {
	if l.changing {
		return body()
	}
	outer, held := l.repo, rawLines(l.Entries)
	first := true
	return outer.Transact(func(tx TimelogRepository) error {
		now, err := tx.Day(l.Date)
		if err != nil {
			return err
		}
		if !first || !slices.Equal(rawLines(now.Entries), held) {
			l.Entries = now.Entries
		}
		first = false
		l.repo, l.changing = tx, true
		defer func() { l.repo, l.changing = outer, false }()
		return body()
	})
}

// ErrFutureEntry is AppendAt's error for a time later than now.
var ErrFutureEntry = errors.New("an entry can't be logged later than now")

// AppendAt logs task at the time at, which is on the log's day, rather
// than now: it annotates task with at under TimelogTimestampLabel, in
// UTC to the second as Append stamps now, puts it in its place in time
// and persists through the wired repo, as Append does. Its place is
// right after the last entry logged at or before at, or first: so in a
// day in time order it lands between the entries before and after it,
// and after any logged at the same time, which, logged earlier, then
// run for no time (see Timesheet). The entries already there keep their
// order, in time or not.
//
// It refuses, changing nothing (neither the log nor task), a time later
// than now (ErrFutureEntry) and one off the log's day. now is the
// current time, and its location the local zone, as for Timesheet: at
// is on the log's day when its date there is the log's Date's year,
// month and day, the date a log's file is named by. An entry without a
// readable ts: is an error too, since the new one can't be placed
// against it.
func (l *Timelog) AppendAt(task *Task, at, now time.Time) error {
	if at.After(now) {
		return fmt.Errorf("%w: %s is after %s", ErrFutureEntry, at.Format(time.RFC3339), now.Format(time.RFC3339))
	}
	// By the date rather than from midnight to midnight: where the
	// clocks go forward at midnight, time.Date puts a midnight that
	// doesn't exist an hour early, on the day before.
	local := at.In(now.Location())
	if y, m, d := local.Date(); y != l.Date.Year() || m != l.Date.Month() || d != l.Date.Day() {
		return fmt.Errorf("%s isn't on the log's day, %s", local.Format(time.RFC3339), l.Date.Format("2006-01-02"))
	}
	return l.transact(func() error {
		place := 0 // the new entry's index
		for i, entry := range l.Entries {
			logged, err := entryTimestamp(entry)
			if err != nil {
				return fmt.Errorf("entry %d: %w", i, err)
			}
			if !logged.After(at) {
				place = i + 1
			}
		}
		if _, err := task.Annotate(TimelogTimestampLabel, at.UTC().Format(time.RFC3339)); err != nil {
			return fmt.Errorf("annotate entry timestamp: %w", err)
		}
		l.Entries = slices.Insert(l.Entries, place, task)
		return l.repo.Save(l)
	})
}
