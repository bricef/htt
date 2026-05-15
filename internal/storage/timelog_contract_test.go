package storage

import (
	"testing"
	"time"

	"github.com/bricef/htt/internal/domain"
)

// runTimelogRepositoryContract exercises behaviours every
// domain.TimelogRepository implementation must satisfy. file_test
// runs the same suite against the file-backed impl in Step 3.
func runTimelogRepositoryContract(t *testing.T, newRepo func(t *testing.T) domain.TimelogRepository) {
	t.Helper()

	t.Run("Today on a fresh store returns an empty timelog (not nil)", func(t *testing.T) {
		r := newRepo(t)
		l, err := r.Today()
		if err != nil {
			t.Fatalf("Today: %v", err)
		}
		if l == nil {
			t.Fatalf("Today returned nil, want empty Timelog")
		}
		if !l.IsEmpty() {
			t.Errorf("fresh repo Today should be empty, got %v entries", len(l.Entries))
		}
	})

	t.Run("Day for a never-saved date returns an empty timelog", func(t *testing.T) {
		r := newRepo(t)
		date := time.Date(2025, 12, 31, 9, 0, 0, 0, time.UTC)
		l, err := r.Day(date)
		if err != nil {
			t.Fatalf("Day: %v", err)
		}
		if !l.IsEmpty() {
			t.Errorf("never-saved date should be empty, got %v entries", len(l.Entries))
		}
		if !l.Date.Equal(date) {
			t.Errorf("Date = %v, want %v", l.Date, date)
		}
	})

	t.Run("Save then Day round-trips entries", func(t *testing.T) {
		r := newRepo(t)
		date := time.Date(2026, 5, 15, 14, 0, 0, 0, time.UTC)

		input := &domain.Timelog{
			Date: date,
			Entries: []*domain.Task{
				mustTask(t, "wrote code ts:2026-05-15T14:00:00Z"),
				mustTask(t, "@end ts:2026-05-15T17:00:00Z"),
			},
		}
		if err := r.Save(input); err != nil {
			t.Fatalf("Save: %v", err)
		}

		loaded, err := r.Day(date)
		if err != nil {
			t.Fatalf("Day: %v", err)
		}
		if len(loaded.Entries) != 2 {
			t.Fatalf("len(Entries) = %d, want 2", len(loaded.Entries))
		}
		if loaded.Entries[0].Raw != input.Entries[0].Raw {
			t.Errorf("Entries[0].Raw = %q, want %q", loaded.Entries[0].Raw, input.Entries[0].Raw)
		}
	})

	t.Run("Save preserves entry order", func(t *testing.T) {
		r := newRepo(t)
		date := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
		raws := []string{
			"first ts:2026-05-15T09:00:00Z",
			"second ts:2026-05-15T10:00:00Z",
			"third ts:2026-05-15T11:00:00Z",
		}
		entries := make([]*domain.Task, len(raws))
		for i, raw := range raws {
			entries[i] = mustTask(t, raw)
		}
		if err := r.Save(&domain.Timelog{Date: date, Entries: entries}); err != nil {
			t.Fatalf("Save: %v", err)
		}

		loaded, err := r.Day(date)
		if err != nil {
			t.Fatalf("Day: %v", err)
		}
		for i, want := range raws {
			if loaded.Entries[i].Raw != want {
				t.Errorf("Entries[%d].Raw = %q, want %q", i, loaded.Entries[i].Raw, want)
			}
		}
	})

	t.Run("Save overwrites prior state for the same date", func(t *testing.T) {
		r := newRepo(t)
		date := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)

		_ = r.Save(&domain.Timelog{
			Date:    date,
			Entries: []*domain.Task{mustTask(t, "v1 ts:2026-05-15T09:00:00Z")},
		})
		_ = r.Save(&domain.Timelog{
			Date: date,
			Entries: []*domain.Task{
				mustTask(t, "v2a ts:2026-05-15T10:00:00Z"),
				mustTask(t, "v2b ts:2026-05-15T11:00:00Z"),
			},
		})

		loaded, _ := r.Day(date)
		if len(loaded.Entries) != 2 {
			t.Fatalf("len(Entries) = %d after overwrite, want 2", len(loaded.Entries))
		}
		if loaded.Entries[0].Raw[:3] != "v2a" {
			t.Errorf("first entry should be from second Save, got %q", loaded.Entries[0].Raw)
		}
	})

	t.Run("Save does not alias caller's Entries slice", func(t *testing.T) {
		r := newRepo(t)
		date := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
		input := &domain.Timelog{
			Date:    date,
			Entries: []*domain.Task{mustTask(t, "original ts:2026-05-15T09:00:00Z")},
		}
		if err := r.Save(input); err != nil {
			t.Fatalf("Save: %v", err)
		}
		input.Entries = append(input.Entries, mustTask(t, "post-save ts:2026-05-15T10:00:00Z"))

		loaded, _ := r.Day(date)
		if len(loaded.Entries) != 1 {
			t.Errorf("post-save mutation leaked into stored state; len = %d, want 1", len(loaded.Entries))
		}
	})

	t.Run("Day for distinct dates returns independent state", func(t *testing.T) {
		r := newRepo(t)
		may15 := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
		may16 := time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)

		_ = r.Save(&domain.Timelog{Date: may15, Entries: []*domain.Task{mustTask(t, "fifteen ts:2026-05-15T09:00:00Z")}})
		_ = r.Save(&domain.Timelog{Date: may16, Entries: []*domain.Task{mustTask(t, "sixteen ts:2026-05-16T09:00:00Z")}})

		l15, _ := r.Day(may15)
		l16, _ := r.Day(may16)
		if len(l15.Entries) != 1 || l15.Entries[0].Raw[:7] != "fifteen" {
			t.Errorf("may15 = %v", l15.Entries)
		}
		if len(l16.Entries) != 1 || l16.Entries[0].Raw[:7] != "sixteen" {
			t.Errorf("may16 = %v", l16.Entries)
		}
	})

	t.Run("Two times on the same calendar day share a Timelog", func(t *testing.T) {
		// The file layout names files by YYYY-MM-DD; the memory impl
		// keys by the same. Two distinct time.Times within one day
		// must map to the same Timelog so morning saves and afternoon
		// saves stack rather than overwrite via key collision.
		r := newRepo(t)
		morning := time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC)
		afternoon := time.Date(2026, 5, 15, 17, 0, 0, 0, time.UTC)

		_ = r.Save(&domain.Timelog{
			Date:    morning,
			Entries: []*domain.Task{mustTask(t, "morning entry ts:2026-05-15T09:00:00Z")},
		})

		loaded, err := r.Day(afternoon)
		if err != nil {
			t.Fatalf("Day(afternoon): %v", err)
		}
		if len(loaded.Entries) != 1 {
			t.Errorf("afternoon Day should see morning save; got %v", loaded.Entries)
		}
	})
}

func TestMemoryTimelogRepository_Contract(t *testing.T) {
	runTimelogRepositoryContract(t, func(t *testing.T) domain.TimelogRepository {
		return NewMemoryTimelogRepository()
	})
}
