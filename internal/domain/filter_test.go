package domain

import (
	"errors"
	"slices"
	"testing"
	"time"
)

// filterNow anchors the due-date filters: 2026-05-16, mid-afternoon in
// the machine's own zone.
var filterNow = time.Date(2026, 5, 16, 15, 0, 0, 0, time.Local)

func TestParseFilter_Matches(t *testing.T) {
	tasks := []*Task{
		mustTask(t, "(A) late due:2026-05-15"),
		mustTask(t, "(B) today due:2026-05-16 @home"),
		mustTask(t, "sixth due:2026-05-22 +home"),
		mustTask(t, "(A) seventh due:2026-05-23 #home"),
		mustTask(t, "undated +htt @homework"),
		mustTask(t, "garbled due:friday"),
	}
	cases := []struct {
		expr string
		want []int
	}{
		{"overdue", []int{0}},
		// [today, today+7d): today counts, the seventh day out does not.
		{"due-soon", []int{1, 2}},
		{"no-due", []int{4, 5}},
		// A bare name matches any sigil but only the whole tag.
		{"has-tag:home", []int{1, 2, 3}},
		{"has-tag:+home", []int{2}},
		{"has-tag:htt", []int{4}},
		{"priority:A", []int{0, 3}},
		{"priority:b", []int{1}},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			f, err := ParseFilter(tc.expr, filterNow)
			if err != nil {
				t.Fatalf("ParseFilter(%q): %v", tc.expr, err)
			}
			got := []int{}
			for i, task := range tasks {
				if f(task) {
					got = append(got, i)
				}
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("%s matched %v, want %v", tc.expr, got, tc.want)
			}
		})
	}
}

func TestParseFilter_UnknownExpressions(t *testing.T) {
	for _, expr := range []string{"", "late", "Overdue", "has-tag:", "priority:", "priority:D", "priority:AB"} {
		_, err := ParseFilter(expr, filterNow)
		if !errors.Is(err, ErrUnknownFilter) {
			t.Errorf("ParseFilter(%q) error = %v, want ErrUnknownFilter", expr, err)
		}
	}
}

func TestTask_Due(t *testing.T) {
	due, ok := mustTask(t, "pay rent due:2026-06-01").Due()
	if !ok || !due.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)) {
		t.Errorf("Due() = %v, %v; want 2026-06-01 local midnight", due, ok)
	}
	for _, raw := range []string{"no due date", "pay rent due:friday", "pay rent due:2026-13-45"} {
		if _, ok := mustTask(t, raw).Due(); ok {
			t.Errorf("%q: Due() ok, want no usable due date", raw)
		}
	}
}

func TestParseLocalDate(t *testing.T) {
	if got, ok := ParseLocalDate("2026-05-16"); !ok || got.Location() != time.Local || got.Day() != 16 {
		t.Errorf("ParseLocalDate(2026-05-16) = %v, %v", got, ok)
	}
	for _, raw := range []string{"", "16/05/2026", "2026-5-16", "tomorrow"} {
		if _, ok := ParseLocalDate(raw); ok {
			t.Errorf("ParseLocalDate(%q) ok, want false", raw)
		}
	}
}
