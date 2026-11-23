package domain

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/bricef/htt/internal/vars"
	"github.com/fatih/color"
)

// Context is a named bundle of Tasks plus the Repository they came from.
// The repo is the seam through which persistent methods (added in Step 3:
// AddTask, Complete, etc.) save changes back. Pure methods (Search, Sort,
// GetTaskById, …) never touch it, so tests for those can construct
// Contexts via &Context{Name: ..., Tasks: ...} struct literals.
type Context struct {
	Name  string
	Tasks []*Task
	repo  Repository

	// doneIDs are the ids of the context's tasks in the done file, as
	// Tree read them, kept until the context is next saved: a command
	// asks for the tree to warn and again to show it.
	doneIDs map[string]bool

	// changing is true while a transaction runs on the context
	// (Transact): its repository is then the transaction's, and a
	// persistent method called on it is part of that transaction.
	changing bool
}

// NewContext returns a Context wired with the given Repository. Intended
// for Repository implementations: external callers obtain Contexts via
// Repository.Context, Repository.Contexts, or Repository.CurrentContext.
// Storage impls construct via this constructor and then populate Tasks
// before returning the Context to a caller.
func NewContext(repo Repository, name string) *Context {
	return &Context{
		repo:  repo,
		Name:  name,
		Tasks: []*Task{},
	}
}

func (c *Context) Equals(other *Context) bool {
	return c.Name == other.Name
}

// Sort orders the tasks by priority, (A) first, then (B) and on, then
// no priority, keeping tasks of the same priority in their order.
func (c *Context) Sort() *Context {
	slices.SortStableFunc(c.Tasks, func(a, b *Task) int {
		return comparePriority(a.Priority, b.Priority)
	})
	return c
}

func (c *Context) GetTaskById(index int) (*Task, error) {
	if len(c.Tasks) == 0 {
		return nil, fmt.Errorf("Task list was empty")
	}
	if index > len(c.Tasks)-1 || index < 0 {
		return nil, fmt.Errorf("item selected is outside of range")
	}
	return c.Tasks[index], nil
}

func (c *Context) GetTaskByStrId(strid string) (*Task, error) {
	id, err := strconv.Atoi(strid)
	if err != nil {
		return nil, fmt.Errorf("Supplied argument '"+strid+"' was not an integer: ", err)
	}
	return c.GetTaskById(id)
}

func (c *Context) GetTaskIndex(task *Task) (int, error) {
	for i, t := range c.Tasks {
		if t == task {
			return i, nil
		}
	}
	return -1, fmt.Errorf("could not find task in context")
}

func (c *Context) ConsoleString() string {
	if vars.GetBool(vars.ConfigKeyDisableColor) {
		return c.Name
	} else {
		return color.GreenString(c.Name)
	}
}

func showTasks(ts []*Task) {
	fmt.Println()
	for _, todo := range ts {
		fmt.Printf("%3d %s\n", todo.Line, todo.ConsoleString())
	}
	fmt.Println()
}

func (c *Context) ShowOnly(predicate func(*Task) bool) {
	ts := c.Search(predicate)
	if len(ts) == 0 {
		fmt.Printf("(%s): No tasks matched query.\n", c.ConsoleString())
		return
	}
	showTasks(ts)
	fmt.Printf("(%s): %d out of %v tasks matched query.\n", c.ConsoleString(), len(ts), len(c.Tasks))
}

func (c *Context) Search(predicate func(*Task) bool) []*Task {
	ts := []*Task{}
	for _, t := range c.Tasks {
		if predicate(t) {
			ts = append(ts, t)
		}
	}
	return ts
}
