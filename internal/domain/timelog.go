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
	startTime, err := entryTimestamp(latest)
	if err != nil {
		return 0, err
	}
	return time.Since(startTime), nil
}

// Span pairs a timelog entry with the duration the user spent on it
// before the next entry was appended. The final entry of a Timelog
// has no Span — it's still in progress.
type Span struct {
	Entry    *Task
	Duration time.Duration
}

// Spans walks consecutive Entries pairwise and reports how long the
// user worked on each before switching. Returns an empty slice when
// the timelog has fewer than two entries (no closed span possible).
// Returns an error if any participating entry's ts: annotation is
// missing or malformed.
//
// Spans deliberately excludes a trailing wall-clock span for the
// last entry. Callers that want "time spent on the currently-active
// entry" can use Duration() (which is wall-clock since Latest's ts:).
// This split matches the Latest()/Duration() pair: Spans is the
// closed-interval view; Duration is the open-interval view.
func (l *Timelog) Spans() ([]Span, error) {
	if len(l.Entries) < 2 {
		return nil, nil
	}
	spans := make([]Span, 0, len(l.Entries)-1)
	for i := 0; i < len(l.Entries)-1; i++ {
		start, err := entryTimestamp(l.Entries[i])
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i, err)
		}
		end, err := entryTimestamp(l.Entries[i+1])
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i+1, err)
		}
		spans = append(spans, Span{Entry: l.Entries[i], Duration: end.Sub(start)})
	}
	return spans, nil
}

// entryTimestamp pulls the ts: annotation off a timelog entry and
// parses it as RFC3339. Shared between Duration and Spans so the
// error wording stays consistent.
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
