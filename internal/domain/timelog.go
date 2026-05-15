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
// (Latest, Duration, IsEmpty) — those never touch the repo. Calling
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

// Latest returns the most recent entry or nil for an empty timelog.
//
// Latest does NOT distinguish "@end" or other sentinel-style entries
// from real activity entries — it returns whatever was last appended.
// `htt log end` writes an @end entry; a subsequent `htt log status`
// reports "Currently working on @end (Xm)" with the time since end
// was called. That's the documented behaviour today; a richer model
// (per-entry Open/Closed) is feature work, not a refactor.
func (l *Timelog) Latest() *Task {
	if len(l.Entries) == 0 {
		return nil
	}
	return l.Entries[len(l.Entries)-1]
}

// Duration returns the wall-clock time since Latest's ts: annotation.
// Returns 0 if the timelog is empty. Returns an error if Latest is
// present but its ts: annotation is missing or malformed.
//
// The legacy code utils.Fatal'd on parse failure; idiomatic returns
// let the CLI propagate the error via RunE the same way as every
// other operation.
func (l *Timelog) Duration() (time.Duration, error) {
	latest := l.Latest()
	if latest == nil {
		return 0, nil
	}
	startedAt, ok := latest.Annotations[TimelogTimestampLabel]
	if !ok {
		return 0, fmt.Errorf("timelog entry missing %s annotation: %q", TimelogTimestampLabel, latest.Raw)
	}
	startTime, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return 0, fmt.Errorf("parse %s=%q: %w", TimelogTimestampLabel, startedAt, err)
	}
	return time.Since(startTime), nil
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
