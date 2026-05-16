package domain_test

import (
	"slices"
	"testing"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/storage"
)

// SwitchableContextNames must hide both reserved contexts — done (for
// completed tasks) and archive (for deleted ones). Neither should
// appear in tab strips, status output, or the TUI context bar.
func TestSwitchableContextNames_HidesReservedContexts(t *testing.T) {
	repo := storage.NewMemoryRepository()

	// Seed via Save so the contexts exist in repo.ContextNames(). Order
	// in the seed is intentionally jumbled — ContextNames sorts.
	for _, name := range []string{"work", domain.DoneContextName, "todo", domain.ArchiveContextName} {
		if err := repo.Save(&domain.Context{Name: name, Tasks: []*domain.Task{}}); err != nil {
			t.Fatalf("Save(%q): %v", name, err)
		}
	}

	got, err := domain.SwitchableContextNames(repo)
	if err != nil {
		t.Fatalf("SwitchableContextNames: %v", err)
	}

	want := []string{"todo", "work"}
	if !slices.Equal(got, want) {
		t.Errorf("SwitchableContextNames = %v, want %v", got, want)
	}
}
