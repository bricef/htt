package domain

import (
	"fmt"
	"regexp"
	"time"
)

// Persistent methods on Context. Each loads any auxiliary context via
// c.repo, applies the in-memory mutation, and saves through c.repo.Save.
// Callers that obtained their Context via Repository.Context (or
// Contexts, CurrentContext) have a wired repo and can use these freely.
// Struct-literal Contexts have c.repo == nil and will nil-deref — that
// is a programmer error, and we want it loud.

// AddTask appends a task to the context and persists. Construction and
// placement are separate concerns: callers do
// `task, _ := domain.NewTask("foo"); ctx.AddTask(task)`.
func (c *Context) AddTask(task *Task) error {
	c.add(task)
	return c.repo.Save(c)
}

// Delete removes the indexed task from the context and persists.
// Returns the deleted task for callers that want to confirm what went.
func (c *Context) Delete(strID string) (*Task, error) {
	task, err := c.GetTaskByStrId(strID)
	if err != nil {
		return nil, err
	}
	if err := c.remove(task); err != nil {
		return nil, err
	}
	if err := c.repo.Save(c); err != nil {
		return nil, err
	}
	return task, nil
}

// Replace swaps the indexed task for replacement, persists, and returns
// the previous task (a fresh parse of the pre-replacement Raw, so callers
// printing a "Before:" line see the un-mutated state).
func (c *Context) Replace(strID string, replacement *Task) (*Task, error) {
	target, err := c.GetTaskByStrId(strID)
	if err != nil {
		return nil, err
	}
	snapshot, err := NewTask(target.Raw)
	if err != nil {
		return nil, fmt.Errorf("snapshot task: %w", err)
	}
	if err := c.replaceInPlace(target, replacement); err != nil {
		return nil, err
	}
	if err := c.repo.Save(c); err != nil {
		return nil, err
	}
	return snapshot, nil
}

// Move transfers the indexed task to another context (loaded via the
// shared repo) and persists both. Returns the moved task.
//
// Save order is destination-first: a partial-save failure (ENOSPC,
// EROFS, EACCES, quota) on the source then leaves the task in *both*
// places — recoverable by re-running the move or deleting one copy.
// Source-first would leave the task in *neither* place, a data loss.
//
// Same-context moves are rejected up front so the load-twice path
// can't duplicate the task into itself.
func (c *Context) Move(strID, toName string) (*Task, error) {
	target, err := c.GetTaskByStrId(strID)
	if err != nil {
		return nil, err
	}
	if toName == c.Name {
		return nil, fmt.Errorf("cannot move task to its current context %q", c.Name)
	}
	to, err := c.repo.Context(toName)
	if err != nil {
		return nil, err
	}
	if err := c.remove(target); err != nil {
		return nil, err
	}
	to.add(target)
	if err := c.repo.Save(to); err != nil {
		return nil, err
	}
	if err := c.repo.Save(c); err != nil {
		return nil, err
	}
	return target, nil
}

// Complete marks the indexed task complete (annotating it with the
// originating context name), moves it into the "done" context, and
// persists both.
//
// Same save-order reasoning as Move: done is saved first so a partial
// failure errs toward duplication over loss. Completing a task that is
// already in the done context is rejected since the task is already
// there and the "originating context" annotation would be misleading.
func (c *Context) Complete(strID string) (*Task, error) {
	target, err := c.GetTaskByStrId(strID)
	if err != nil {
		return nil, err
	}
	if c.Name == DoneContextName {
		return nil, fmt.Errorf("cannot complete a task already in %q", DoneContextName)
	}
	done, err := c.repo.Context(DoneContextName)
	if err != nil {
		return nil, err
	}
	if _, err := target.markCompleted(c, time.Now()); err != nil {
		return nil, fmt.Errorf("mark task complete: %w", err)
	}
	if err := c.remove(target); err != nil {
		return nil, err
	}
	done.add(target)
	if err := c.repo.Save(done); err != nil {
		return nil, err
	}
	if err := c.repo.Save(c); err != nil {
		return nil, err
	}
	return target, nil
}

// SetPriority assigns priority to the indexed task and persists. Returns
// (snapshot, mutated) so the CLI can show "Before:" / "After:". Returns
// an error for priorities outside the valid range [A-C].
//
// The regex matches Task.setPriority's actual supported range. Wider
// regexes (e.g. [A-F]) pass validation here but get silently coerced to
// empty by setPriority — making a typo look like a clean priority-clear,
// which is data loss without a diagnostic.
func (c *Context) SetPriority(strID, priority string) (*Task, *Task, error) {
	if !validPriorityRE.MatchString(priority) {
		return nil, nil, fmt.Errorf("invalid priority %q (must be one of A, B, C)", priority)
	}
	return c.priorityTransform(strID, func(t *Task) *Task { return t.setPriority(priority) })
}

// IncreasePriority raises the indexed task's priority by one step and persists.
func (c *Context) IncreasePriority(strID string) (*Task, *Task, error) {
	return c.priorityTransform(strID, func(t *Task) *Task { return t.increasePriority() })
}

// DecreasePriority lowers the indexed task's priority by one step and persists.
func (c *Context) DecreasePriority(strID string) (*Task, *Task, error) {
	return c.priorityTransform(strID, func(t *Task) *Task { return t.decreasePriority() })
}

// priorityTransform centralises the snapshot-mutate-sort-save pattern
// shared by SetPriority / IncreasePriority / DecreasePriority.
//
// Snapshot must happen before transform: the Task mutators
// (setPriority / increasePriority / decreasePriority) mutate the
// receiver in place and return the same pointer, so without a
// pre-snapshot a "Before:" view would render the post-mutation state.
//
// Sort must happen before Save: priority is a sort key, and the
// on-disk file is the source of truth on the next reload (which both
// CLI re-invocations and the TUI's refresh path use). Sorting only the
// in-memory copy after Save would let the displayed order desync from
// the disk order on the next refresh.
func (c *Context) priorityTransform(strID string, transform func(*Task) *Task) (*Task, *Task, error) {
	target, err := c.GetTaskByStrId(strID)
	if err != nil {
		return nil, nil, err
	}
	snapshot, err := NewTask(target.Raw)
	if err != nil {
		return nil, nil, fmt.Errorf("snapshot task: %w", err)
	}
	mutated := transform(target)
	c.Sort()
	if err := c.repo.Save(c); err != nil {
		return nil, nil, err
	}
	return snapshot, mutated, nil
}

// add appends a task to the in-memory slice. Internal helper used by the
// persistent methods; not exported so external callers must go through
// AddTask (which persists).
func (c *Context) add(t *Task) {
	c.Tasks = append(c.Tasks, t)
}

// remove drops a task pointer from the in-memory slice by identity.
// Returns an error if the task isn't present. Internal helper.
func (c *Context) remove(target *Task) error {
	for i, t := range c.Tasks {
		if t == target {
			c.Tasks = append(c.Tasks[:i], c.Tasks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("could not find task %v in context %q", target.Raw, c.Name)
}

// replaceInPlace swaps old for new in the in-memory slice. Internal
// helper used by Replace.
func (c *Context) replaceInPlace(old, new *Task) error {
	idx, err := c.GetTaskIndex(old)
	if err != nil {
		return err
	}
	c.Tasks[idx] = new
	return nil
}

var validPriorityRE = regexp.MustCompile(`^[ABC]$`)
