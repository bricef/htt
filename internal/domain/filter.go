package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrUnknownFilter is returned by ParseFilter for an expression it
// doesn't recognise. Callers word the error for their own surface
// (the CLI's --filter flag, the TUI's search box).
var ErrUnknownFilter = errors.New("unknown filter")

// Filter is a predicate over tasks, parsed from a filter expression.
type Filter func(*Task) bool

// ParseFilter turns one filter expression into a Filter:
//
//	overdue        has a due date before today
//	due-soon       has a due date in [today, today+7d)
//	no-due         has no usable due date
//	has-tag:NAME   carries the tag (see Task.HasTag)
//	priority:X     has priority A, B or C (case-insensitive)
//
// Due dates are compared as calendar days against now's local date.
// A due: value that isn't a YYYY-MM-DD date counts as no due date (see
// Task.Due), so such tasks surface under no-due.
func ParseFilter(expr string, now time.Time) (Filter, error) {
	today := startOfDay(now)
	switch expr {
	case "overdue":
		return func(t *Task) bool {
			due, ok := t.Due()
			return ok && due.Before(today)
		}, nil
	case "due-soon":
		horizon := today.AddDate(0, 0, 7)
		return func(t *Task) bool {
			due, ok := t.Due()
			return ok && !due.Before(today) && due.Before(horizon)
		}, nil
	case "no-due":
		return func(t *Task) bool {
			_, ok := t.Due()
			return !ok
		}, nil
	}
	if name, ok := strings.CutPrefix(expr, "has-tag:"); ok && name != "" {
		return func(t *Task) bool { return t.HasTag(name) }, nil
	}
	if letter, ok := strings.CutPrefix(expr, "priority:"); ok {
		p := strings.ToUpper(letter)
		if p == "A" || p == "B" || p == "C" {
			return func(t *Task) bool { return t.Priority == p }, nil
		}
	}
	return nil, fmt.Errorf("%w %q", ErrUnknownFilter, expr)
}

// Due returns the task's due: annotation as a local date, or false
// when it is missing or malformed.
func (t *Task) Due() (time.Time, bool) {
	return ParseLocalDate(t.Annotations["due"])
}

// HasTag reports whether the task carries the tag name. Tag values
// keep their sigil ("@home"), so a bare name matches any sigil and a
// sigiled name ("+home") matches only that kind.
func (t *Task) HasTag(name string) bool {
	for sigil, values := range t.Tags {
		for _, v := range values {
			if v == name || v == sigil+name {
				return true
			}
		}
	}
	return false
}

// ParseLocalDate parses a YYYY-MM-DD value (an annotation, say) as a
// date in local time. It returns false for empty or malformed input,
// which callers treat as "no date" rather than an error.
func ParseLocalDate(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02", raw, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
