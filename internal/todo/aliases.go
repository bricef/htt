package todo

// Aliases bridging the legacy todo package to the new domain package.
// External callers (CLI, TUI) continue to reference todo.Task / todo.Context /
// todo.NewTask / todo.NewContext unchanged. The underlying types and pure
// value-level constructors now live in internal/domain. The package-level
// functions in todo.go (GetCurrentContext, Move, CompleteTask, ...) that
// mix viper config and on-disk state stay here until they move to the
// usecase layer.

import "github.com/bricef/htt/internal/domain"

type (
	Task    = domain.Task
	Context = domain.Context
)

var (
	NewTask    = domain.NewTask
	NewContext = domain.NewContext
)
