package storage

import (
	"fmt"
	"slices"
	"sort"

	"github.com/bricef/htt/internal/domain"
)

// MemoryRepository is an in-memory domain.Repository implementation for
// tests. Not safe for concurrent use; tests don't need it.
//
// It keeps copies: Save stores a copy of each task (re-parsed from its
// Raw) and Context hands out copies, as the file repository's
// write-then-read does, so a change to a task loaded from it reaches
// the store only through a save. Its transactions (Transact) hold their
// saves until the change returns, and run the change again when the
// store changed meanwhile, which Underneath lets a test do: no lock,
// no files.
type MemoryRepository struct {
	contexts map[string][]*domain.Task
	current  string
	changes  int // the store's changes, for a transaction's check

	// Underneath, when set, runs in each transaction after its change
	// and before its check, with the run's number from 1, as another
	// program changing the files then would: anything it saves makes
	// the transaction run its change again, on the store as it is
	// then, up to the file repository's number of runs. For tests.
	Underneath func(run int)
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		contexts: map[string][]*domain.Task{},
	}
}

func (r *MemoryRepository) ContextNames() ([]string, error) {
	names := make([]string, 0, len(r.contexts))
	for name := range r.contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (r *MemoryRepository) Context(name string) (*domain.Context, error) {
	return r.load(r, name, r.contexts[name])
}

// load makes the named context of stored, copied, wired to repo: r, or
// a transaction of r's. It's arranged, as FileRepository loads a
// context.
func (r *MemoryRepository) load(repo domain.Repository, name string, stored []*domain.Task) (*domain.Context, error) {
	if name == "" {
		return nil, domain.ErrInvalidContextName
	}
	tasks, err := cloneTasks(stored)
	if err != nil {
		return nil, fmt.Errorf("copy %s: %w", name, err)
	}
	ctx := domain.NewContext(repo, name)
	ctx.Tasks = tasks
	ctx.Arrange()
	return ctx, nil
}

// ContextLines returns the named context's lines (domain.LineSource).
func (r *MemoryRepository) ContextLines(name string) ([]string, error) {
	if name == "" {
		return nil, domain.ErrInvalidContextName
	}
	return taskLines(r.contexts[name]), nil
}

func taskLines(tasks []*domain.Task) []string {
	var lines []string
	for _, t := range tasks {
		lines = append(lines, t.Raw)
	}
	return lines
}

func (r *MemoryRepository) Contexts() ([]*domain.Context, error) {
	names, err := r.ContextNames()
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Context, 0, len(names))
	for _, name := range names {
		ctx, err := r.Context(name)
		if err != nil {
			return nil, err
		}
		out = append(out, ctx)
	}
	return out, nil
}

// Save stores a copy of ctx's tasks, at once: it changes the store
// under any transaction running, as another htt's save would.
func (r *MemoryRepository) Save(ctx *domain.Context) error {
	tasks, err := saved(ctx)
	if err != nil {
		return err
	}
	r.contexts[ctx.Name] = tasks
	r.changes++
	return nil
}

// saved is a copy of ctx's tasks to store.
func saved(ctx *domain.Context) ([]*domain.Task, error) {
	if ctx == nil || ctx.Name == "" {
		return nil, domain.ErrInvalidContextName
	}
	tasks, err := cloneTasks(ctx.Tasks)
	if err != nil {
		return nil, fmt.Errorf("copy %s: %w", ctx.Name, err)
	}
	return tasks, nil
}

func (r *MemoryRepository) CurrentContextName() (string, error) {
	if r.current == "" {
		return domain.DefaultContextName, nil
	}
	return r.current, nil
}

func (r *MemoryRepository) CurrentContext() (*domain.Context, error) {
	name, err := r.CurrentContextName()
	if err != nil {
		return nil, err
	}
	return r.Context(name)
}

func (r *MemoryRepository) SetCurrent(name string) error {
	sanitized := domain.ContextFileName(name)
	if sanitized == "" {
		return domain.ErrInvalidContextName
	}
	r.current = sanitized
	r.changes++
	return nil
}

// ContextPath returns the empty string for the in-memory repo; the
// path concept is meaningless without a filesystem. The CLI's
// `htt todo edit-done` and the TUI's EditFile action treat an empty
// path as "not supported by this repo". Production runs with
// FileRepository where the path is real.
func (r *MemoryRepository) ContextPath(name string) string {
	return ""
}

// Transact runs change as one transaction (domain.Repository.Transact):
// change gets a memoryTx, whose saves are held until it returns; then,
// unless the store has changed since the run began (Underneath, or a
// save straight to r), they're stored, in order. Otherwise change runs
// again, as many times as the file repository runs a change, then
// gives up with domain.ErrKeepsChanging.
func (r *MemoryRepository) Transact(change func(tx domain.Repository) error) error {
	for run := 1; ; run++ {
		before := r.changes
		tx := &memoryTx{r: r}
		err := change(tx)
		tx.over = true
		if err != nil {
			return err
		}
		if len(tx.order) == 0 && tx.current == "" {
			return nil
		}
		if r.Underneath != nil {
			r.Underneath(run)
		}
		if r.changes == before {
			for _, name := range tx.order {
				r.contexts[name] = tx.saved[name]
			}
			if tx.current != "" {
				r.current = tx.current
			}
			r.changes++
			return nil
		}
		if run == runs {
			return fmt.Errorf("the memory store %w", domain.ErrKeepsChanging)
		}
	}
}

// memoryTx is a MemoryRepository within one run of a transaction's
// change: it reads the store, or what the change saved, and holds what
// it saves. Once the change has returned it saves nothing, and Transact
// on it starts a transaction of its own.
type memoryTx struct {
	r       *MemoryRepository
	saved   map[string][]*domain.Task
	order   []string // the names saved, in order
	current string   // the pointer set, if it was
	over    bool
}

var (
	_ domain.Repository = (*memoryTx)(nil)
	_ domain.LineSource = (*memoryTx)(nil)
)

func (tx *memoryTx) stored(name string) []*domain.Task {
	if tasks, ok := tx.saved[name]; ok {
		return tasks
	}
	return tx.r.contexts[name]
}

func (tx *memoryTx) Context(name string) (*domain.Context, error) {
	return tx.r.load(tx, name, tx.stored(name))
}

func (tx *memoryTx) ContextLines(name string) ([]string, error) {
	if name == "" {
		return nil, domain.ErrInvalidContextName
	}
	return taskLines(tx.stored(name)), nil
}

func (tx *memoryTx) ContextNames() ([]string, error) {
	names, err := tx.r.ContextNames()
	if err != nil {
		return nil, err
	}
	for _, name := range tx.order {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func (tx *memoryTx) Contexts() ([]*domain.Context, error) {
	names, err := tx.ContextNames()
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Context, 0, len(names))
	for _, name := range names {
		ctx, err := tx.Context(name)
		if err != nil {
			return nil, err
		}
		out = append(out, ctx)
	}
	return out, nil
}

func (tx *memoryTx) CurrentContextName() (string, error) {
	if tx.current != "" {
		return tx.current, nil
	}
	return tx.r.CurrentContextName()
}

func (tx *memoryTx) CurrentContext() (*domain.Context, error) {
	name, err := tx.CurrentContextName()
	if err != nil {
		return nil, err
	}
	return tx.Context(name)
}

func (tx *memoryTx) SetCurrent(name string) error {
	sanitized := domain.ContextFileName(name)
	if sanitized == "" {
		return domain.ErrInvalidContextName
	}
	if tx.over {
		return errOver
	}
	tx.current = sanitized
	return nil
}

func (tx *memoryTx) Save(ctx *domain.Context) error {
	if tx.over {
		return errOver
	}
	tasks, err := saved(ctx)
	if err != nil {
		return err
	}
	if tx.saved == nil {
		tx.saved = map[string][]*domain.Task{}
	}
	if _, ok := tx.saved[ctx.Name]; !ok {
		tx.order = append(tx.order, ctx.Name)
	}
	tx.saved[ctx.Name] = tasks
	return nil
}

func (tx *memoryTx) ContextPath(string) string { return "" }

// Transact runs change as part of this transaction, while its change
// runs; afterwards, as a transaction of its own.
func (tx *memoryTx) Transact(change func(tx domain.Repository) error) error {
	if tx.over {
		return tx.r.Transact(change)
	}
	return change(tx)
}
