package domain

import (
	"testing"
	"time"
)

// stubTimelogRepository is a placeholder TimelogRepository for the
// NewTimelog wiring test. Methods panic if invoked: only identity is
// asserted here.
type stubTimelogRepository struct{}

func (*stubTimelogRepository) Today() (*Timelog, error)        { panic("stub") }
func (*stubTimelogRepository) Day(time.Time) (*Timelog, error) { panic("stub") }
func (*stubTimelogRepository) Save(*Timelog) error             { panic("stub") }
func (*stubTimelogRepository) CurrentLogPath() string          { panic("stub") }
func (*stubTimelogRepository) LogPath(time.Time) string        { panic("stub") }
func (*stubTimelogRepository) Transact(func(TimelogRepository) error) error {
	panic("stub")
}

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

// timestampedEntry builds a Task whose ts: annotation is set to a
// fixed time, as Timelog.Append stamps it.
func timestampedEntry(t *testing.T, body string, ts time.Time) *Task {
	t.Helper()
	entry := mustTask(t, body)
	if _, err := entry.Annotate(TimelogTimestampLabel, ts.Format(time.RFC3339)); err != nil {
		t.Fatalf("Annotate: %v", err)
	}
	return entry
}
