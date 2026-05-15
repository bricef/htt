package domain

import "errors"

// Repository is the domain abstraction for persistence. Storage
// implementations satisfy this interface; the domain package owns the
// contract.
//
// External callers obtain Contexts through this interface — never via
// domain.NewContext directly. The repo is the factory: Context, Contexts,
// and CurrentContext return Contexts wired with this repo so their
// persistent methods (AddTask, Complete, etc.) can save through it.
type Repository interface {
	// Context returns the named context, loaded with its tasks. A name
	// that has never been persisted returns an empty Context (Tasks
	// empty), not an error.
	Context(name string) (*Context, error)

	// Contexts returns every persisted context with tasks loaded. Heavier
	// than ContextNames; call this only when the task lists matter (e.g.
	// cumulative stats across contexts).
	Contexts() ([]*Context, error)

	// ContextNames returns the names of every persisted context. Cheap.
	// Use for tab strips, status output, anywhere a name list is enough.
	// An empty store returns ([]string{}, nil), not nil.
	ContextNames() ([]string, error)

	// CurrentContext returns the active context, loaded. Equivalent to
	// Context(CurrentContextName()) but expressed as one call.
	CurrentContext() (*Context, error)

	// CurrentContextName returns the name of the active context. If no
	// pointer has been set, returns DefaultContextName.
	CurrentContextName() (string, error)

	// SetCurrent persists the active-context pointer. Implementations
	// sanitize the name (non-word characters become underscores) before
	// persistence. Returns ErrInvalidContextName if the name sanitizes
	// to empty.
	SetCurrent(name string) error

	// Save persists a context, overwriting any prior state for the same
	// name. Intended for internal use by Context's mutation methods
	// (AddTask, Delete, etc.); external callers should mutate through
	// those methods rather than calling Save directly. It is exported
	// because Go's unexported-method interface matching only works
	// within a single package, and storage implementations live in a
	// separate package.
	Save(ctx *Context) error
}

// ErrInvalidContextName is returned when a name fails validation (empty,
// or sanitizes to empty).
var ErrInvalidContextName = errors.New("invalid context name")

// DefaultContextName is the name returned by CurrentContextName when no
// pointer has been set yet. Matches vars.DefaultContext.
const DefaultContextName = "todo"

// DoneContextName is the conventional target for completed tasks.
// Context.Complete moves the indexed task into this context.
const DoneContextName = "done"

// SwitchableContextNames returns every persisted context name except
// DoneContextName — the contexts a user can usefully switch to. Order
// matches Repository.ContextNames.
//
// This is a presentation-layer helper hoisted into the domain package so
// CLI and TUI share a single definition (avoids a tiny six-line filter
// loop at each call site).
func SwitchableContextNames(r Repository) ([]string, error) {
	all, err := r.ContextNames()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(all))
	for _, n := range all {
		if n != DoneContextName {
			out = append(out, n)
		}
	}
	return out, nil
}
