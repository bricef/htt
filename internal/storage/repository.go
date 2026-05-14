// Package storage defines the Repository abstraction that the usecase layer
// uses to read and write task contexts and the current-context pointer.
//
// Two implementations live in this package: an in-memory fake (memory.go)
// for fast, deterministic tests, and a file-backed implementation
// (file.go, added in Step 6) that owns the on-disk layout. Both pass the
// same contract_test.go suite.
package storage

import (
	"errors"

	"github.com/bricef/htt/internal/domain"
)

// Repository is the boundary between the business layer and persistence.
// Implementations must satisfy the contract pinned by contract_test.go.
type Repository interface {
	// ListContexts returns the names of every persisted context, including
	// the conventional "done" context. Order is implementation-defined.
	// An empty store returns ([]string{}, nil), not nil.
	ListContexts() ([]string, error)

	// LoadContext returns the named context. A context that has never been
	// saved returns an empty Context (Tasks is empty), not an error.
	LoadContext(name string) (*domain.Context, error)

	// SaveContext persists the context, overwriting any prior state for the
	// same name.
	SaveContext(ctx *domain.Context) error

	// GetCurrentContextName returns the name of the active context. If no
	// current-context pointer has been set, returns "todo" (the default).
	GetCurrentContextName() (string, error)

	// SetCurrentContextName persists the active-context pointer.
	SetCurrentContextName(name string) error
}

// ErrInvalidContextName is returned when a name fails validation (e.g. empty).
var ErrInvalidContextName = errors.New("invalid context name")

// DefaultContextName is the name returned by GetCurrentContextName when no
// pointer has been set yet. Matches vars.DefaultContext.
const DefaultContextName = "todo"
