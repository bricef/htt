// Package usecase wraps storage.Repository with the business operations
// that CLI and TUI presentation code call into. Use cases return domain
// values and errors; they do not print, exit, or read viper config.
package usecase

import (
	"fmt"
	"regexp"
	"time"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/storage"
	"github.com/bricef/htt/internal/utils"
)

// DoneContextName is the conventional target for completed tasks.
const DoneContextName = "done"

// UseCases bundles every business operation against a single Repository.
type UseCases struct {
	repo storage.Repository
}

func New(repo storage.Repository) *UseCases {
	return &UseCases{repo: repo}
}

// AddTask appends a new task to the current context.
func (u *UseCases) AddTask(raw string) (*domain.Task, *domain.Context, error) {
	name, err := u.repo.GetCurrentContextName()
	if err != nil {
		return nil, nil, err
	}
	return u.AddTaskTo(name, raw)
}

// AddTaskTo appends a new task to the named context.
func (u *UseCases) AddTaskTo(contextName, raw string) (*domain.Task, *domain.Context, error) {
	task, err := domain.NewTask(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("parse task: %w", err)
	}
	ctx, err := u.repo.LoadContext(contextName)
	if err != nil {
		return nil, nil, err
	}
	ctx.Tasks = append(ctx.Tasks, task)
	if err := u.repo.SaveContext(ctx); err != nil {
		return nil, nil, err
	}
	return task, ctx, nil
}

// CompleteTask marks the indexed task in the current context complete,
// annotates it with the originating context, and moves it to "done".
func (u *UseCases) CompleteTask(strID string) (*domain.Task, error) {
	currentName, err := u.repo.GetCurrentContextName()
	if err != nil {
		return nil, err
	}
	current, err := u.repo.LoadContext(currentName)
	if err != nil {
		return nil, err
	}
	task, err := current.GetTaskByStrId(strID)
	if err != nil {
		return nil, err
	}

	done, err := u.repo.LoadContext(DoneContextName)
	if err != nil {
		return nil, err
	}

	if _, err := task.Do(current, time.Now()); err != nil {
		return nil, fmt.Errorf("mark task complete: %w", err)
	}

	if err := removeTask(current, task); err != nil {
		return nil, err
	}
	done.Tasks = append(done.Tasks, task)

	if err := u.repo.SaveContext(current); err != nil {
		return nil, err
	}
	if err := u.repo.SaveContext(done); err != nil {
		return nil, err
	}
	return task, nil
}

// DeleteTask removes the indexed task from the current context.
func (u *UseCases) DeleteTask(strID string) (*domain.Task, error) {
	currentName, err := u.repo.GetCurrentContextName()
	if err != nil {
		return nil, err
	}
	ctx, err := u.repo.LoadContext(currentName)
	if err != nil {
		return nil, err
	}
	task, err := ctx.GetTaskByStrId(strID)
	if err != nil {
		return nil, err
	}
	if err := removeTask(ctx, task); err != nil {
		return nil, err
	}
	if err := u.repo.SaveContext(ctx); err != nil {
		return nil, err
	}
	return task, nil
}

// MoveTask moves the indexed task from the current context to another.
func (u *UseCases) MoveTask(strID, toContextName string) (*domain.Task, string, string, error) {
	fromName, err := u.repo.GetCurrentContextName()
	if err != nil {
		return nil, "", "", err
	}
	from, err := u.repo.LoadContext(fromName)
	if err != nil {
		return nil, "", "", err
	}
	to, err := u.repo.LoadContext(toContextName)
	if err != nil {
		return nil, "", "", err
	}
	task, err := from.GetTaskByStrId(strID)
	if err != nil {
		return nil, "", "", err
	}
	if err := removeTask(from, task); err != nil {
		return nil, "", "", err
	}
	to.Tasks = append(to.Tasks, task)

	if err := u.repo.SaveContext(from); err != nil {
		return nil, "", "", err
	}
	if err := u.repo.SaveContext(to); err != nil {
		return nil, "", "", err
	}
	return task, fromName, toContextName, nil
}

// ReplaceTask swaps the indexed task in the current context for a new
// entry built from raw.
func (u *UseCases) ReplaceTask(strID, raw string) (*domain.Task, *domain.Task, error) {
	replacement, err := domain.NewTask(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("parse replacement task: %w", err)
	}
	return u.swapTask(strID, func(_ *domain.Task) (*domain.Task, error) {
		return replacement, nil
	})
}

// SetPriority sets an explicit priority letter on the indexed task in
// the current context.
func (u *UseCases) SetPriority(strID, priority string) (*domain.Task, *domain.Task, error) {
	if !validPriorityRE.MatchString(priority) {
		return nil, nil, fmt.Errorf("invalid priority %q", priority)
	}
	return u.swapTask(strID, func(t *domain.Task) (*domain.Task, error) {
		return t.SetPriority(priority), nil
	})
}

// IncreasePriority raises the indexed task's priority by one step.
func (u *UseCases) IncreasePriority(strID string) (*domain.Task, *domain.Task, error) {
	return u.swapTask(strID, func(t *domain.Task) (*domain.Task, error) {
		return t.IncreasePriority(), nil
	})
}

// DecreasePriority lowers the indexed task's priority by one step.
func (u *UseCases) DecreasePriority(strID string) (*domain.Task, *domain.Task, error) {
	return u.swapTask(strID, func(t *domain.Task) (*domain.Task, error) {
		return t.DecreasePriority(), nil
	})
}

// CurrentContext returns the active context with its tasks loaded.
func (u *UseCases) CurrentContext() (*domain.Context, error) {
	name, err := u.repo.GetCurrentContextName()
	if err != nil {
		return nil, err
	}
	return u.repo.LoadContext(name)
}

// CurrentContextName returns just the name of the active context.
func (u *UseCases) CurrentContextName() (string, error) {
	return u.repo.GetCurrentContextName()
}

// SwitchContext makes the named context active. The name is sanitized
// to a valid filename component (non-word characters become underscores),
// matching legacy todo.SetCurrentContext behavior. Returns the sanitized
// name actually persisted.
func (u *UseCases) SwitchContext(rawName string) (string, error) {
	name := utils.StringToFilename(rawName)
	if name == "" {
		return "", storage.ErrInvalidContextName
	}
	if err := u.repo.SetCurrentContextName(name); err != nil {
		return "", err
	}
	return name, nil
}

// SearchCurrentContext returns the current context plus the subset of
// tasks whose raw text matches pattern (interpreted as a case-insensitive
// Go regular expression).
func (u *UseCases) SearchCurrentContext(pattern string) (*domain.Context, []*domain.Task, error) {
	ctx, err := u.CurrentContext()
	if err != nil {
		return nil, nil, err
	}
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid pattern: %w", err)
	}
	matches := ctx.Search(func(t *domain.Task) bool {
		return re.MatchString(t.Raw)
	})
	return ctx, matches, nil
}

// ListContextNames returns the names of every persisted context except
// the conventional "done" target. The done filter matches the legacy
// todo.GetContexts behavior used by `htt todo status`. Callers that need
// the full set (including done) can call storage.Repository.ListContexts
// directly.
func (u *UseCases) ListContextNames() ([]string, error) {
	all, err := u.repo.ListContexts()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(all))
	for _, name := range all {
		if name == DoneContextName {
			continue
		}
		out = append(out, name)
	}
	return out, nil
}

// swapTask loads the current context, finds the task by string ID, applies
// transform to produce a new task, replaces in-place, persists, and returns
// (snapshotOfOriginal, new).
//
// The pre-mutation snapshot is critical: Task.SetPriority /
// IncreasePriority / DecreasePriority all mutate the receiver in place and
// return the same pointer. Without snapshotting first, callers that want to
// print a "Before:" view (the CLI does) would see the post-mutation state.
func (u *UseCases) swapTask(strID string, transform func(*domain.Task) (*domain.Task, error)) (*domain.Task, *domain.Task, error) {
	currentName, err := u.repo.GetCurrentContextName()
	if err != nil {
		return nil, nil, err
	}
	ctx, err := u.repo.LoadContext(currentName)
	if err != nil {
		return nil, nil, err
	}
	target, err := ctx.GetTaskByStrId(strID)
	if err != nil {
		return nil, nil, err
	}
	old, err := domain.NewTask(target.Raw)
	if err != nil {
		return nil, nil, fmt.Errorf("snapshot task: %w", err)
	}
	newTask, err := transform(target)
	if err != nil {
		return nil, nil, err
	}
	if err := ctx.Replace(target, newTask); err != nil {
		return nil, nil, err
	}
	if err := u.repo.SaveContext(ctx); err != nil {
		return nil, nil, err
	}
	return old, newTask, nil
}

// removeTask drops the given task pointer from ctx.Tasks in place, without
// invoking the I/O-bound Context.Remove method (which still calls Sync on
// the legacy domain type until Step 10).
func removeTask(ctx *domain.Context, target *domain.Task) error {
	for i, t := range ctx.Tasks {
		if t == target {
			ctx.Tasks = append(ctx.Tasks[:i], ctx.Tasks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("task not found in context %q", ctx.Name)
}

var validPriorityRE = regexp.MustCompile(`^[ABCDEF]$`)
