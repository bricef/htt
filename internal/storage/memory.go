package storage

import (
	"sort"

	"github.com/bricef/htt/internal/domain"
)

// MemoryRepository is an in-memory Repository implementation for tests.
// Not safe for concurrent use; tests don't need it.
type MemoryRepository struct {
	contexts map[string][]*domain.Task
	current  string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		contexts: map[string][]*domain.Task{},
	}
}

func (r *MemoryRepository) ListContexts() ([]string, error) {
	names := make([]string, 0, len(r.contexts))
	for name := range r.contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (r *MemoryRepository) LoadContext(name string) (*domain.Context, error) {
	if name == "" {
		return nil, ErrInvalidContextName
	}
	stored := r.contexts[name]
	tasks := make([]*domain.Task, len(stored))
	copy(tasks, stored)
	return &domain.Context{Name: name, Tasks: tasks}, nil
}

func (r *MemoryRepository) SaveContext(ctx *domain.Context) error {
	if ctx == nil || ctx.Name == "" {
		return ErrInvalidContextName
	}
	tasks := make([]*domain.Task, len(ctx.Tasks))
	copy(tasks, ctx.Tasks)
	r.contexts[ctx.Name] = tasks
	return nil
}

func (r *MemoryRepository) GetCurrentContextName() (string, error) {
	if r.current == "" {
		return DefaultContextName, nil
	}
	return r.current, nil
}

func (r *MemoryRepository) SetCurrentContextName(name string) error {
	if name == "" {
		return ErrInvalidContextName
	}
	r.current = name
	return nil
}
