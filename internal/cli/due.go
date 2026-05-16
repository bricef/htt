package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/olebedev/when"
)

// parseDue accepts an absolute YYYY-MM-DD date or a natural-language
// phrase ("Friday", "tomorrow", "in two weeks", "next Monday") and
// returns the resolved local date with the time-of-day stripped.
// `now` anchors relative phrases; tests inject a fixed clock.
//
// The natural-language path delegates to olebedev/when's English
// parser, which carries enough coverage for daily-use phrasing.
// A no-match from when becomes an error so the CLI can surface
// "didn't understand <input>" rather than silently store the wrong
// date.
//
// All resolved dates are truncated to start-of-day in the local
// timezone. Annotations on a Task only carry a date, not a
// time-of-day, and the rest of htt's date plumbing (CreatedOn,
// CompletedOn, deleted-on) treats midnight-local as the canonical
// form.
func parseDue(input string, now time.Time) (time.Time, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return time.Time{}, fmt.Errorf("empty due value")
	}
	if t, err := time.ParseInLocation("2006-01-02", input, time.Local); err == nil {
		return t, nil
	}
	r, err := when.EN.Parse(input, now)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %q: %w", input, err)
	}
	if r == nil {
		return time.Time{}, fmt.Errorf("could not understand %q (try YYYY-MM-DD or phrases like \"Friday\", \"tomorrow\", \"in two weeks\")", input)
	}
	loc := now.Location()
	t := r.Time.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc), nil
}
