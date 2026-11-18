package domain

import (
	"fmt"
	"time"
)

// Timelog is one day's worth of activity entries. Each entry is a
// Task annotated with a "ts:<RFC3339>" timestamp by Append.
//
// The repo seam is the same pattern as Context: Timelogs handed out
// by a TimelogRepository carry a wired repo and can use Append (which
// saves). Struct-literal Timelogs work for pure-method tests
// (IsEmpty, Timesheet) — those never touch the repo. Calling
// Append on a struct-literal Timelog will nil-deref; that's a
// programmer error, and we want it loud.
type Timelog struct {
	Date    time.Time
	Entries []*Task
	repo    TimelogRepository
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
func (l *Timelog) Append(task *Task) error {
	now := time.Now().UTC()
	if _, err := task.Annotate(TimelogTimestampLabel, now.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("annotate entry timestamp: %w", err)
	}
	l.Entries = append(l.Entries, task)
	return l.repo.Save(l)
}
