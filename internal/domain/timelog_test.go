package domain

import (
	"strings"
	"testing"
	"time"
)

// stubTimelogRepository is a placeholder TimelogRepository for the
// NewTimelog wiring test. Methods panic if invoked: only identity is
// asserted here.
type stubTimelogRepository struct{}

func (*stubTimelogRepository) Today() (*Timelog, error)         { panic("stub") }
func (*stubTimelogRepository) Day(time.Time) (*Timelog, error)  { panic("stub") }
func (*stubTimelogRepository) Save(*Timelog) error              { panic("stub") }
func (*stubTimelogRepository) CurrentLogPath() string           { panic("stub") }

func TestNewTimelog_InjectsRepo(t *testing.T) {
	// Mirrors TestNewContext_InjectsRepo: NewTimelog must store the
	// supplied repo on the Timelog. Without it, Append nil-derefs.
	stub := &stubTimelogRepository{}
	date := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	l := NewTimelog(stub, date)

	if l.repo != TimelogRepository(stub) {
		t.Errorf("l.repo did not get the supplied repo")
	}
	if !l.Date.Equal(date) {
		t.Errorf("l.Date = %v, want %v", l.Date, date)
	}
	if len(l.Entries) != 0 {
		t.Errorf("l.Entries should start empty, got %v", l.Entries)
	}
}

func TestTimelog_IsEmpty(t *testing.T) {
	l := &Timelog{}
	if !l.IsEmpty() {
		t.Errorf("fresh timelog should be empty")
	}
	l.Entries = append(l.Entries, mustTask(t, "hello"))
	if l.IsEmpty() {
		t.Errorf("timelog with one entry should not be empty")
	}
}

func TestTimelog_Latest_ReturnsLastOrNil(t *testing.T) {
	l := &Timelog{}
	if l.Latest() != nil {
		t.Errorf("empty timelog Latest should be nil")
	}

	l.Entries = []*Task{
		mustTask(t, "first"),
		mustTask(t, "second"),
		mustTask(t, "third"),
	}
	got := l.Latest()
	if got == nil || got.Raw != "third" {
		t.Errorf("Latest = %v, want third", got)
	}
}

func TestTimelog_Latest_DoesNotSkipEndSentinel(t *testing.T) {
	// Documented behaviour: Latest returns whatever was last appended,
	// including @end. The CLI's `log status` therefore reports
	// "Currently working on @end (...)" after a `log end` — which is
	// the current product behaviour (preserved by the refactor; a
	// richer sentinel-handling model is feature work).
	l := &Timelog{Entries: []*Task{
		mustTask(t, "writing code"),
		mustTask(t, "@end"),
	}}
	got := l.Latest()
	if got == nil || got.Raw != "@end" {
		t.Errorf("Latest = %v, want @end (preserved naive semantics)", got)
	}
}

func TestTimelog_Duration_ZeroWhenEmpty(t *testing.T) {
	l := &Timelog{}
	got, err := l.Duration()
	if err != nil {
		t.Errorf("Duration on empty timelog should not error, got %v", err)
	}
	if got != 0 {
		t.Errorf("Duration on empty timelog = %v, want 0", got)
	}
}

func TestTimelog_Duration_MeasuresFromLatestTimestamp(t *testing.T) {
	// Construct a task with a ts: annotation set to ~now-5s. Duration
	// should land in a sensible range around 5s. A 0.5s tolerance
	// absorbs scheduler jitter; a fixed clock isn't worth the
	// injectable-now complexity for this single test.
	want := 5 * time.Second
	stamp := time.Now().UTC().Add(-want).Format(time.RFC3339)

	entry := mustTask(t, "doing a thing")
	if _, err := entry.Annotate(TimelogTimestampLabel, stamp); err != nil {
		t.Fatalf("Annotate: %v", err)
	}

	l := &Timelog{Entries: []*Task{entry}}
	got, err := l.Duration()
	if err != nil {
		t.Fatalf("Duration: %v", err)
	}
	delta := got - want
	if delta < 0 {
		delta = -delta
	}
	if delta > time.Second {
		t.Errorf("Duration = %v, want ~%v (delta = %v)", got, want, delta)
	}
}

func TestTimelog_Duration_ErrorOnMissingAnnotation(t *testing.T) {
	// A bare task with no ts: annotation must surface an error rather
	// than the silent 0 the legacy utils.Fatal path would produce.
	entry := mustTask(t, "no timestamp here")
	l := &Timelog{Entries: []*Task{entry}}

	_, err := l.Duration()
	if err == nil {
		t.Errorf("Duration should error when ts: annotation missing")
	}
	if err != nil && !strings.Contains(err.Error(), TimelogTimestampLabel) {
		t.Errorf("error should mention ts annotation key, got %v", err)
	}
}

func TestTimelog_Duration_ErrorOnMalformedTimestamp(t *testing.T) {
	entry := mustTask(t, "broken")
	if _, err := entry.Annotate(TimelogTimestampLabel, "not-a-timestamp"); err != nil {
		t.Fatalf("Annotate: %v", err)
	}
	l := &Timelog{Entries: []*Task{entry}}

	_, err := l.Duration()
	if err == nil {
		t.Errorf("Duration should error on malformed timestamp")
	}
}
