package domain

import (
	"errors"
	"slices"
	"strconv"
)

// Following a task (docs/design/safe-writes.md, "Limits"): a task
// number is read when a command starts. A command that then waits for
// the user (the CLI's `todo edit` while the editor is open, its
// question about open subtasks, the TUI's actions on the task it
// showed) remembers the task's line too, and when the user answers,
// finds the task by it in the file as it is then: another htt may have
// added or removed a task above it meanwhile. A task that changed or
// went meanwhile is left alone.

var (
	// ErrTaskChanged: the task a command waited on the user for isn't
	// in the file as it was (FindTask), changed or removed elsewhere.
	ErrTaskChanged = errors.New("the task changed or went elsewhere")
	// ErrSubtasksChanged: the open subtasks a question asked about
	// aren't the task's open subtasks any more.
	ErrSubtasksChanged = errors.New("its open subtasks changed elsewhere")
)

// FindTask finds, among tasks (a context's, as its file holds them
// now), the task that was at index as raw: still at index, or, if the
// file moved it, the one task with that line. -1 when it isn't there,
// or several tasks have its line and none is at index.
func FindTask(tasks []*Task, index int, raw string) int {
	if index >= 0 && index < len(tasks) && tasks[index].Raw == raw {
		return index
	}
	at := -1
	for i, t := range tasks {
		if t.Raw == raw {
			if at >= 0 {
				return -1
			}
			at = i
		}
	}
	return at
}

// ReplaceFound replaces the task that was at index as raw, found in the
// context as the store holds it now (FindTask), with replacement, and
// persists, as one transaction. It returns the task replaced, as
// Replace does. When the task isn't there, nothing is saved, and the
// error is ErrTaskChanged.
func (c *Context) ReplaceFound(index int, raw string, replacement *Task) (*Task, error) {
	var before *Task
	err := c.Transact(func(c *Context) error {
		at := FindTask(c.Tasks, index, raw)
		if at < 0 {
			return ErrTaskChanged
		}
		var err error
		before, err = c.Replace(strconv.Itoa(at), replacement)
		return err
	})
	if err != nil {
		return nil, err
	}
	return before, nil
}

// CompleteFound completes, as the user answered the question about its
// open subtasks (choice), the task that was at index as raw when it was
// asked, found in the context as the store holds it now (FindTask),
// provided its open subtasks are the ones asked about (open, their
// lines, in tree order), as one transaction. It returns the tasks
// completed, as Complete does. When the task isn't there, the error is
// ErrTaskChanged; when its open subtasks aren't those, it's
// ErrSubtasksChanged; nothing is completed.
func (c *Context) CompleteFound(index int, raw string, open []string, choice Subtasks) ([]*Task, error) {
	var done []*Task
	err := c.Transact(func(c *Context) error {
		at := FindTask(c.Tasks, index, raw)
		if at < 0 {
			return ErrTaskChanged
		}
		now, err := c.OpenSubtasks(strconv.Itoa(at))
		if err != nil {
			return err
		}
		if !slices.Equal(rawLines(now), open) {
			return ErrSubtasksChanged
		}
		done, err = c.Complete(strconv.Itoa(at), choice)
		return err
	})
	if err != nil {
		return nil, err
	}
	return done, nil
}
