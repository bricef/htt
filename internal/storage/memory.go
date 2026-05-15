package storage

import (
	"sort"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/utils"
)

// MemoryRepository is an in-memory domain.Repository implementation for
// tests. Not safe for concurrent use; tests don't need it.
type MemoryRepository struct {
	contexts map[string][]*domain.Task
	current  string
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
	if name == "" {
		return nil, domain.ErrInvalidContextName
	}
	stored := r.contexts[name]
	ctx := domain.NewContext(r, name)
	ctx.Tasks = make([]*domain.Task, len(stored))
	copy(ctx.Tasks, stored)
	return ctx, nil
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

func (r *MemoryRepository) Save(ctx *domain.Context) error {
	if ctx == nil || ctx.Name == "" {
		return domain.ErrInvalidContextName
	}
	tasks := make([]*domain.Task, len(ctx.Tasks))
	copy(tasks, ctx.Tasks)
	r.contexts[ctx.Name] = tasks
	return nil
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
	sanitized := utils.StringToFilename(name)
	if sanitized == "" {
		return domain.ErrInvalidContextName
	}
	r.current = sanitized
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
