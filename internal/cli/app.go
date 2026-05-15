package cli

import (
	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/storage"
	"github.com/bricef/htt/internal/vars"
)

// defaultRepo is the domain.Repository CLI commands reach for. Lazily
// initialized from viper config on first use; tests override it via
// SetRepository to inject a memory-backed repo.
var defaultRepo domain.Repository

func repo() domain.Repository {
	if defaultRepo == nil {
		defaultRepo = storage.NewFileRepository(vars.Get(vars.ConfigKeyDataDir))
	}
	return defaultRepo
}

// SetRepository overrides the package-level Repository used by every CLI
// command. Pass nil to reset and force lazy reconstruction on next call.
// Used by tests; main.go does not need to call this.
func SetRepository(r domain.Repository) {
	defaultRepo = r
}
