package domain

import (
	"fmt"
	"time"
)

// TimelogRepository abstracts per-day timelog persistence. Storage
// implementations satisfy this interface; the domain package owns the
// contract.
//
// Separate from Repository because contexts and timelogs are
// independent persistence concerns — they may live under different
// roots on disk and have different file layouts. External callers
// obtain Timelogs through this interface; the constructor is intended
// for repository implementations only.
type TimelogRepository interface {
	// Today returns the current day's Timelog, loaded with its entries.
	// A day that has never been written to returns an empty Timelog
	// (Entries empty), not an error.
	Today() (*Timelog, error)

	// Day returns the Timelog for the given date. Same empty-Timelog
	// semantics for dates that have never been written.
	Day(date time.Time) (*Timelog, error)

	// Save persists a Timelog, overwriting any prior state for the
	// same date. Intended for internal use by Timelog.Append and
	// AppendAt; external callers should mutate through Timelog
	// methods.
	Save(l *Timelog) error

	// CurrentLogPath returns the on-disk path for today's log file.
	// Pure path builder — performs no I/O. Used by `htt log edit` to
	// hand a path to $EDITOR; not intended for programmatic reads or
	// writes (those go through Today / Day / Save / Timelog.Append /
	// AppendAt).
	CurrentLogPath() string

	// LogPath returns the on-disk path for date's log file, "" for a
	// repository without files. Pure path builder, like
	// CurrentLogPath; the CLI names it in messages about an entry.
	LogPath(date time.Time) string
}

// LastLogBefore is the log of the most recent of the n days before
// now's that has entries: yesterday's, or failing that the day
// before's, and so on; nil when none of them has an entry. It loads
// each day through r.Day until one has entries, and none further back.
//
// now's location is the local zone, as for Timelog.Timesheet: the days
// are calendar days there, each one back by its date, as a log's file
// is named, not 24 hours back (a day the clocks change is 23 or 25
// hours long).
func LastLogBefore(r TimelogRepository, now time.Time, n int) (*Timelog, error) {
	y, m, d := now.Date()
	for back := 1; back <= n; back++ {
		// Noon on the day: where the clocks go forward at midnight, that
		// midnight doesn't exist, and time.Date puts it on the day before.
		day := time.Date(y, m, d-back, 12, 0, 0, 0, now.Location())
		l, err := r.Day(day)
		if err != nil {
			return nil, fmt.Errorf("load the timelog for %s: %w", day.Format("2006-01-02"), err)
		}
		if !l.IsEmpty() {
			return l, nil
		}
	}
	return nil, nil
}

// TimelogTimestampLabel is the annotation key Timelog.Append and
// AppendAt set on each entry with the RFC3339-formatted time it was
// logged at, in UTC. The CLI strips this annotation for display in
// `htt log show` and `htt log status`.
const TimelogTimestampLabel = "ts"
