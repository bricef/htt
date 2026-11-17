package domain

import (
	"slices"
	"testing"
)

// A line with a date htt can't read still loads: the date is text (or,
// for due:, the task has no due date). DateWarnings says so, for the
// CLI and the TUI to show; it never stops a line loading.

func TestDateWarnings_PointsOutDatesHttCantRead(t *testing.T) {
	cases := []struct {
		raw  string
		want []string
	}{
		// Impossible dates, wherever they are.
		{"2026-13-45 call the bank", []string{"2026-13-45 isn't a real date, so it's text"}},
		{"(A) 2026-02-30 call the bank", []string{"2026-02-30 isn't a real date, so it's text"}},
		{"x 2026-05-01 2026-02-30 called", []string{"2026-02-30 isn't a real date, so it's text"}},
		{"call the bank on 2026-02-30", []string{"2026-02-30 isn't a real date, so it's text"}},
		// Dates not written YYYY-MM-DD.
		{"dentist 2026-5-3", []string{"2026-5-3 isn't written YYYY-MM-DD (2026-05-03), so it's text"}},
		{"2026-11-3 dentist", []string{"2026-11-3 isn't written YYYY-MM-DD (2026-11-03), so it's text"}},
		{"dentist 2026-2-30", []string{"2026-2-30 isn't a real date, so it's text"}},
		// due: values that aren't dates.
		{"pay rent due:2026-02-30", []string{"due:2026-02-30 isn't a real date, so the task has no due date"}},
		{"pay rent due:2026-5-3", []string{"due:2026-5-3 isn't written YYYY-MM-DD (2026-05-03), so the task has no due date"}},
		{"pay rent due:friday", []string{"due:friday isn't a YYYY-MM-DD date, so the task has no due date"}},
		// Only the due: Task.Due reads, the last one.
		{"pay due:friday then due:2026-05-03", nil},
		{"pay due:2026-05-03 then due:friday", []string{"due:friday isn't a YYYY-MM-DD date, so the task has no due date"}},
		// Several, in line order.
		{"2026-13-45 then due:soon and 2026-5-3", []string{
			"2026-13-45 isn't a real date, so it's text",
			"due:soon isn't a YYYY-MM-DD date, so the task has no due date",
			"2026-5-3 isn't written YYYY-MM-DD (2026-05-03), so it's text",
		}},
		// Real dates, and words that aren't dates.
		{"(A) 2026-05-03 call on 2026-05-04 due:2026-05-05", nil},
		{"x 2026-05-03 2026-05-01 done thing", nil},
		{"2026-05 20260503 26-05-03 12026-05-03 2026-05-03, v1.2.3", nil},
		{"+2026-13-45 @2026-13-45 #2026-13-45 t:2026-13-45 ts:2026-13-45T10:00", nil},
		{"meet at 10:30 2026/05/03", nil},
	}
	for _, tc := range cases {
		task := mustTask(t, tc.raw)
		if got := task.DateWarnings(); !slices.Equal(got, tc.want) {
			t.Errorf("DateWarnings(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestDateWarnings_TheLineStillLoadsWithEveryWord(t *testing.T) {
	task := mustTask(t, "2026-13-45 call the bank due:friday")
	if !task.CreatedOn.IsZero() {
		t.Errorf("2026-13-45 read as the created date %v", task.CreatedOn)
	}
	if got, want := task.Entry(), "2026-13-45 call the bank"; got != want {
		t.Errorf("Entry() = %q, want %q", got, want)
	}
	if _, ok := task.Due(); ok {
		t.Errorf("due:friday read as a due date")
	}
	if got := task.Annotations["due"]; got != "friday" {
		t.Errorf("due annotation = %q, want friday", got)
	}
	if len(task.DateWarnings()) != 2 {
		t.Errorf("want two warnings, got %q", task.DateWarnings())
	}
}
