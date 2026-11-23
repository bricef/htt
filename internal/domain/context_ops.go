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
//
// If the task has no explicit CreatedOn (the user didn't include a
// `YYYY-MM-DD` prefix in the input), AddTask stamps it with the
// current local date so a later `htt report` can list "what was
// added this week". An explicit date from the parser is preserved.
func (c *Context) AddTask(task *Task) error {
	task.StampCreated(time.Now())
	c.add(task)
	return c.save()
}

// Delete moves the indexed task, with every task below it, to the
// archive context (annotating each with archived-from:<source> and
// deleted-on:<date>) and persists both. Returns the tasks moved, the
// indexed one first, then its subtree in tree order, so callers can
// confirm what went.
//
// Deleting from the archive itself is true removal — the task is
// dropped from the in-memory slice and the archive is re-saved.
// Otherwise we'd loop forever (move into the same file, re-annotate,
// never actually drop). This is the seam for manual garbage
// collection: open archive.txt in $EDITOR or call `htt todo delete`
// while in the archive context.
//
// Same save-order reasoning as Move / Complete: archive is saved
// first so a partial failure (ENOSPC, EROFS, quota) errs toward
// duplication over loss — the user can rerun the delete or remove
// the duplicate, but a lost task is gone.
func (c *Context) Delete(strID string) ([]*Task, error) {
	tasks, err := c.below(strID)
	if err != nil {
		return nil, err
	}
	if c.Name == ArchiveContextName {
		if err := c.remove(tasks[0]); err != nil {
			return nil, err
		}
		if err := c.save(); err != nil {
			return nil, err
		}
		return tasks, nil
	}
	archive, err := c.repo.Context(ArchiveContextName)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for _, t := range tasks {
		if _, err := t.markDeleted(c, now); err != nil {
			return nil, fmt.Errorf("mark task deleted: %w", err)
		}
		if err := c.remove(t); err != nil {
			return nil, err
		}
		archive.add(t)
	}
	if err := archive.save(); err != nil {
		return nil, err
	}
	if err := c.save(); err != nil {
		return nil, err
	}
	return tasks, nil
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
	if err := c.save(); err != nil {
		return nil, err
	}
	return snapshot, nil
}

// Move transfers the indexed task, with every task below it, to
// another context (loaded via the shared repo) and persists both.
// Returns the tasks moved, the indexed one first, then its subtree in
// tree order. Into a context file, the subtree fits in (fitInto): the
// indexed task loses its p:, since a parent is always in its subtasks'
// file, and ids the target already uses are renumbered, keeping the
// subtree's links; a done task going back to the context it was
// completed in links up again instead. Into the done file or the
// archive, the lines go as they are.
//
// Save order is destination-first: a partial-save failure (ENOSPC,
// EROFS, EACCES, quota) on the source then leaves the task in *both*
// places — recoverable by re-running the move or deleting one copy.
// Source-first would leave the task in *neither* place, a data loss.
//
// Same-context moves are rejected up front so the load-twice path
// can't duplicate the task into itself, or save the context without
// the subtree over the file it was just written to. Names are compared
// as files are named (ContextFileName): "my list" is my_list.
func (c *Context) Move(strID, toName string) ([]*Task, error) {
	tasks, err := c.below(strID)
	if err != nil {
		return nil, err
	}
	if ContextFileName(toName) == ContextFileName(c.Name) {
		if toName != c.Name {
			return nil, fmt.Errorf("cannot move task to its current context %q: %q names its file", c.Name, toName)
		}
		return nil, fmt.Errorf("cannot move task to its current context %q", c.Name)
	}
	to, err := c.repo.Context(toName)
	if err != nil {
		return nil, err
	}
	if to.IsTree() {
		if err := c.fitInto(tasks, to); err != nil {
			return nil, err
		}
	}
	for _, t := range tasks {
		if err := c.remove(t); err != nil {
			return nil, err
		}
		to.add(t)
	}
	if err := to.save(); err != nil {
		return nil, err
	}
	if err := c.save(); err != nil {
		return nil, err
	}
	return tasks, nil
}

// Complete marks the indexed task complete (annotating it with the
// originating context name), moves it into the "done" context, and
// persists both. A task with open tasks below it, at any depth, takes
// a choice (Subtasks): SubtasksAsk refuses with ErrOpenSubtasks,
// changing nothing; SubtasksLeave completes the task alone, its
// subtasks staying open and naming it; SubtasksComplete completes
// every open task below it too. Returns the tasks completed, the
// indexed one first, then the rest in tree order.
//
// Same save-order reasoning as Move: done is saved first so a partial
// failure errs toward duplication over loss. Completing a task that is
// already in the done context is rejected since the task is already
// there and the "originating context" annotation would be misleading.
func (c *Context) Complete(strID string, subtasks Subtasks) ([]*Task, error) {
	target, err := c.GetTaskByStrId(strID)
	if err != nil {
		return nil, err
	}
	if c.Name == DoneContextName {
		return nil, fmt.Errorf("cannot complete a task already in %q", DoneContextName)
	}
	open, err := c.OpenSubtasks(strID)
	if err != nil {
		return nil, err
	}
	tasks := []*Task{target}
	switch {
	case len(open) == 0 || subtasks == SubtasksLeave:
	case subtasks == SubtasksComplete:
		tasks = append(tasks, open...)
	default:
		return nil, fmt.Errorf("task %s: %w (%d)", strID, ErrOpenSubtasks, len(open))
	}
	done, err := c.repo.Context(DoneContextName)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for _, t := range tasks {
		if _, err := t.markCompleted(c, now); err != nil {
			return nil, fmt.Errorf("mark task complete: %w", err)
		}
		if err := c.remove(t); err != nil {
			return nil, err
		}
		done.add(t)
	}
	if err := done.save(); err != nil {
		return nil, err
	}
	if err := c.save(); err != nil {
		return nil, err
	}
	return tasks, nil
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
	if err := c.save(); err != nil {
		return nil, nil, err
	}
	return snapshot, mutated, nil
}

// save writes the context through its repository, arranged first
// (Arrange): when htt writes a context file, every subtree is directly
// below its parent. Every persistent method saves through it.
func (c *Context) save() error {
	c.Arrange()
	c.doneIDs = nil // a completion may have added to them
	return c.repo.Save(c)
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
