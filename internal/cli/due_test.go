package cli

import (
	"testing"
	"time"
)

// anchored picks a fixed Friday so weekday-relative phrases ("next
// Monday") have a deterministic answer regardless of when the test
// runs.
var anchored = time.Date(2026, 5, 15, 10, 0, 0, 0, time.Local) // a Friday

func TestParseDue_AbsoluteDate(t *testing.T) {
	got, err := parseDue("2026-05-22", anchored)
	if err != nil {
		t.Fatalf("parseDue: %v", err)
	}
	want := time.Date(2026, 5, 22, 0, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseDue_RelativePhrases(t *testing.T) {
	// "tomorrow" anchored on 2026-05-15 should resolve to 2026-05-16.
	// "in two weeks" should resolve to 2026-05-29.
	// "next Monday" from a Friday should jump to 2026-05-18 (the
	// upcoming Monday) — what users intuitively expect.
	cases := []struct {
		input string
		want  time.Time
	}{
		{"tomorrow", time.Date(2026, 5, 16, 0, 0, 0, 0, time.Local)},
		{"in two weeks", time.Date(2026, 5, 29, 0, 0, 0, 0, time.Local)},
		{"in 3 days", time.Date(2026, 5, 18, 0, 0, 0, 0, time.Local)},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got, err := parseDue(c.input, anchored)
			if err != nil {
				t.Fatalf("parseDue(%q): %v", c.input, err)
			}
			if !got.Equal(c.want) {
				t.Errorf("parseDue(%q) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}

func TestParseDue_TruncatesTimeOfDay(t *testing.T) {
	// "tomorrow at 3pm" carries a time-of-day; the helper must
	// strip it so the annotation value remains date-only.
	got, err := parseDue("tomorrow at 3pm", anchored)
	if err != nil {
		t.Fatalf("parseDue: %v", err)
	}
	if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 {
		t.Errorf("expected midnight, got %v", got)
	}
}

func TestParseDue_RejectsEmpty(t *testing.T) {
	if _, err := parseDue("", anchored); err == nil {
		t.Errorf("empty input should error")
	}
	if _, err := parseDue("   ", anchored); err == nil {
		t.Errorf("whitespace-only input should error")
	}
}

func TestParseDue_RejectsNonsense(t *testing.T) {
	// Plain gibberish should error rather than silently picking
	// a date — the user typed something the parser doesn't know,
	// they need a diagnostic.
	if _, err := parseDue("xyzzy", anchored); err == nil {
		t.Errorf("nonsense input should error")
	}
}
