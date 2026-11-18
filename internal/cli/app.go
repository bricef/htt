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

// repo returns the Repository CLI commands use: defaultRepo, warning
// on stderr about the dates htt can't read in each context a command
// loads (see warningRepository).
func repo() domain.Repository {
	return warningRepository{baseRepo()}
}

// baseRepo returns defaultRepo itself, without the warnings. The TUI
// gets this one: it owns the terminal, and shows date warnings on its
// status line.
func baseRepo() domain.Repository {
	if defaultRepo == nil {
		// Pointer file lives under tracker_path to preserve the legacy
		// layout for users that overrode tracker_path independently of
		// data_path. Default config maps both to the same directory.
		defaultRepo = storage.NewFileRepository(
			vars.Get(vars.ConfigKeyDataDir),
			vars.Get(vars.ConfigKeyTrackerDir),
		)
	}
	return defaultRepo
}

// SetRepository overrides the package-level Repository used by every CLI
// command. Pass nil to reset and force lazy reconstruction on next call.
// Used by tests; main.go does not need to call this.
func SetRepository(r domain.Repository) {
	defaultRepo = r
}

// defaultTimelogRepo is the domain.TimelogRepository CLI `log` and
// `workon` commands reach for. Lazily initialized from viper config
// on first use; tests override it via SetTimelogRepository.
var defaultTimelogRepo domain.TimelogRepository

// timelogRepo returns the TimelogRepository CLI commands use:
// defaultTimelogRepo, warning on stderr about the dates htt can't read
// in each day's log a command loads (see warningTimelogRepository).
func timelogRepo() domain.TimelogRepository {
	if defaultTimelogRepo == nil {
		defaultTimelogRepo = storage.NewFileTimelogRepository(vars.Get(vars.ConfigKeyDataDir))
	}
	return warningTimelogRepository{defaultTimelogRepo}
}

// SetTimelogRepository overrides the package-level TimelogRepository
// used by every CLI `log`/`workon` command. Pass nil to reset.
// Used by tests; main.go does not need to call this.
func SetTimelogRepository(r domain.TimelogRepository) {
	defaultTimelogRepo = r
}
