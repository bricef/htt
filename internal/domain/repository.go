package domain

import (
	"errors"
	"regexp/syntax"
	"strings"
)

// Repository is the domain abstraction for persistence. Storage
// implementations satisfy this interface; the domain package owns the
// contract.
//
// External callers obtain Contexts through this interface — never via
// domain.NewContext directly. The repo is the factory: Context, Contexts,
// and CurrentContext return Contexts wired with this repo so their
// persistent methods (AddTask, Complete, etc.) can save through it.
type Repository interface {
	// Context returns the named context, loaded with its tasks,
	// arranged (Context.Arrange): in tree order, numbered in it,
	// whatever order the file holds them in. A name that has never been
	// persisted returns an empty Context (Tasks empty), not an error.
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

	// ContextPath returns the on-disk path for the named context's
	// file. Pure path builder — performs no I/O. Used by
	// `htt todo edit-done` and the TUI's EditFile action to hand a
	// path to $EDITOR; not intended for programmatic reads or writes
	// (those go through Context / Save / Context.AddTask / etc).
	//
	// In-memory implementations have no real path and may return the
	// empty string; callers that depend on a usable path should
	// check for it.
	ContextPath(name string) string
}

// LineSource is a Repository that can also hand over a context's lines
// unparsed, as Context loads them (trimmed, blank lines left out).
// Context.Tree uses it to read the done file for done parents, parsing
// only the lines that could hold one: parsing every line of a large
// done file would take seconds. Both of storage's repositories are one;
// Tree falls back to Context for any other.
type LineSource interface {
	ContextLines(name string) ([]string, error)
}

// SameFiler is a Repository that can tell whether two contexts are kept
// in one file, which comparing their names (ContextFileName) can't: on a
// case-insensitive file system, macOS's and Windows' default, todo.txt
// and TODO.txt are one file, and a symlink makes any two names one.
// Context.Move refuses to move a task between them, which would write
// it into the file and then save that file without it.
type SameFiler interface {
	SameFile(a, b string) bool
}

// ErrInvalidContextName is returned when a name fails validation (empty,
// or sanitizes to empty).
var ErrInvalidContextName = errors.New("invalid context name")

// ContextFileName is the name a context's file takes, without its
// extension: the context's name with every character but an ASCII
// letter, a digit or an underscore made an underscore. Names that map
// to the same file name are one context ("my list" is my_list).
// Repositories name files, and the current-context pointer, by it.
func ContextFileName(name string) string {
	return strings.Map(func(r rune) rune {
		if syntax.IsWordChar(r) {
			return r
		}
		return '_'
	}, name)
}

// DefaultContextName is the name returned by CurrentContextName when no
// pointer has been set yet. Matches vars.DefaultContext.
const DefaultContextName = "todo"

// DoneContextName is the conventional target for completed tasks.
// Context.Complete moves the indexed task into this context.
const DoneContextName = "done"

// ArchiveContextName is the conventional target for deleted tasks.
// Context.Delete moves the indexed task into this context, annotating
// it with archived-from:<source> and deleted-on:<date>. Deleting from
// the archive itself is true removal (no further archival).
const ArchiveContextName = "archive"

// SwitchableContextNames returns every persisted context name except
// the reserved ones (done, archive) — the contexts a user can
// usefully switch to. Order matches Repository.ContextNames.
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
		if n == DoneContextName || n == ArchiveContextName {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}
