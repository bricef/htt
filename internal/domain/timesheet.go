package domain

import (
	"fmt"
	"slices"
	"time"
)

// DayEndEntry is the entry `htt log end` writes. It ends the day; see
// Timelog.Timesheet.
const DayEndEntry = "@end"

// IsDayEnd reports whether entry ends the day: its text, leaving out
// annotations such as ts:, a priority and dates, is exactly @end. An
// entry that has the tag among other words ("fix the @end bug") is an
// ordinary entry.
func IsDayEnd(entry *Task) bool {
	return entry.Entry() == DayEndEntry
}

// DayStatus is how a day's time log stands at its end, or now.
type DayStatus int

const (
	DayEmpty      DayStatus = iota // no entries
	DayEnded                       // the last entry is an @end
	DayInProgress                  // today, and the last entry is still running
	DayUnended                     // a past day whose last entry isn't an @end: it ran to 24:00
)

func (s DayStatus) String() string {
	switch s {
	case DayEmpty:
		return "empty"
	case DayEnded:
		return "ended"
	case DayInProgress:
		return "in progress"
	case DayUnended:
		return "unended"
	}
	return fmt.Sprintf("DayStatus(%d)", int(s))
}

// Interval is the time one entry ran, from Start to End, in its
// category.
type Interval struct {
	Entry      *Task
	Category   Category
	Start, End time.Time
}

// Duration is how long the interval ran.
func (iv Interval) Duration() time.Duration {
	return iv.End.Sub(iv.Start)
}

// Timesheet is a day's time log read as intervals: what ran when, and
// in which category.
type Timesheet struct {
	Start, End time.Time  // the day: local midnight to the next local midnight
	Intervals  []Interval // in time order, one per entry that isn't an @end
	Status     DayStatus
	EndedAt    time.Time // DayEnded: the time of the @end that ended the day
}

// Timesheet reads the day's entries as intervals, each entry in its
// category from c (nil for none: everything is working).
//
// An entry runs from its ts: to the next entry's: the last entry before
// an @end runs to the @end, and an @end makes no interval, so the gap
// after it, until the next entry, belongs to nothing. The day's last
// entry runs to 24:00 (the day is DayUnended) unless it is an @end
// (DayEnded); except today, whose last entry runs until now and is
// DayInProgress.
//
// now is the current time, and its location the local time zone: the
// day runs from midnight to midnight there, on the log's date (its
// Date's year, month and day). An interval never leaves its day: an
// entry from before midnight (a hand edit, a change of zone) counts
// from midnight, one after the day ends counts for nothing, and today
// nothing counts after now. Entries are taken in time order, those
// logged at the same time in file order, the earlier ones running for
// no time. A missing or malformed ts: is an error.
func (l *Timelog) Timesheet(c *Categoriser, now time.Time) (*Timesheet, error) {
	loc := now.Location()
	start := time.Date(l.Date.Year(), l.Date.Month(), l.Date.Day(), 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1) // 23 or 25 hours away on a day the clocks change
	sheet := &Timesheet{Start: start, End: end}
	if len(l.Entries) == 0 {
		return sheet, nil
	}

	type stamped struct {
		entry *Task
		at    time.Time
	}
	entries := make([]stamped, len(l.Entries))
	for i, entry := range l.Entries {
		at, err := entryTimestamp(entry)
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i, err)
		}
		entries[i] = stamped{entry, at}
	}
	slices.SortStableFunc(entries, func(a, b stamped) int { return a.at.Compare(b.at) })

	today := !now.Before(start) && now.Before(end)
	limit := end
	if today {
		limit = now
	}
	within := func(at time.Time) time.Time {
		switch {
		case at.Before(start):
			return start
		case at.After(limit):
			return limit
		}
		return at
	}
	for i, e := range entries {
		if IsDayEnd(e.entry) {
			continue
		}
		until := limit
		if i+1 < len(entries) {
			until = within(entries[i+1].at)
		}
		sheet.Intervals = append(sheet.Intervals, Interval{
			Entry:    e.entry,
			Category: c.Categorise(e.entry),
			Start:    within(e.at),
			End:      until,
		})
	}

	last := entries[len(entries)-1]
	switch {
	case IsDayEnd(last.entry):
		sheet.Status, sheet.EndedAt = DayEnded, last.at
	case today:
		sheet.Status = DayInProgress
	default:
		sheet.Status = DayUnended
	}
	return sheet, nil
}

// Worked is the time in categories that count as work.
func (s *Timesheet) Worked() time.Duration {
	var d time.Duration
	for _, iv := range s.Intervals {
		if iv.Category.Work {
			d += iv.Duration()
		}
	}
	return d
}

// ByCategory is the time in each category with an interval, breaks
// included, by category name.
func (s *Timesheet) ByCategory() map[string]time.Duration {
	by := map[string]time.Duration{}
	for _, iv := range s.Intervals {
		by[iv.Category.Name] += iv.Duration()
	}
	return by
}

// Current is the interval running now, today's last one, or nil when
// nothing is in progress.
func (s *Timesheet) Current() *Interval {
	if s.Status != DayInProgress {
		return nil
	}
	return &s.Intervals[len(s.Intervals)-1]
}
