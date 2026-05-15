package domain

import "time"

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
	// same date. Intended for internal use by Timelog.Append;
	// external callers should mutate through Timelog methods.
	Save(l *Timelog) error

	// CurrentLogPath returns the on-disk path for today's log file.
	// Pure path builder — performs no I/O. Used by `htt log edit` to
	// hand a path to $EDITOR; not intended for programmatic reads or
	// writes (those go through Today / Day / Save / Timelog.Append).
	CurrentLogPath() string
}

// TimelogTimestampLabel is the annotation key Timelog.Append sets on
// each entry with the RFC3339-formatted append time. The CLI strips
// this annotation for display in `htt log show` and `htt log status`.
const TimelogTimestampLabel = "ts"
