package cli

import (
	"github.com/bricef/htt/internal/storage"
	"github.com/bricef/htt/internal/usecase"
	"github.com/bricef/htt/internal/vars"
)

// defaultUC is the use case set the commands reach for. Initialized lazily
// from viper config on first use; can be overridden by tests via
// SetUseCases to inject a memory-backed repository.
var defaultUC *usecase.UseCases

func uc() *usecase.UseCases {
	if defaultUC == nil {
		defaultUC = usecase.New(storage.NewFileRepository(vars.Get(vars.ConfigKeyDataDir)))
	}
	return defaultUC
}

// SetUseCases overrides the package-level UseCases used by every CLI
// command. Pass nil to reset and force lazy reconstruction on next call.
// Used by tests; main.go does not need to call this.
func SetUseCases(u *usecase.UseCases) {
	defaultUC = u
}
