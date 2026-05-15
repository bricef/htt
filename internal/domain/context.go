package domain

import (
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/bricef/htt/internal/vars"
	"github.com/fatih/color"
)

type Context struct {
	Name  string
	Tasks []*Task
}

// NewContext returns an empty Context with the given name. Loading tasks
// from storage is the repository's job (domain.Repository.Context).
func NewContext(name string) *Context {
	return &Context{
		Name:  name,
		Tasks: []*Task{},
	}
}

func (c *Context) Equals(other *Context) bool {
	return c.Name == other.Name
}

// Add appends a task to the in-memory context. Persistence is the caller's
// responsibility, via domain.Repository.Save.
func (c *Context) Add(t *Task) *Context {
	c.Tasks = append(c.Tasks, t)
	return c
}

// Remove deletes a task pointer from the in-memory context. Persistence is
// the caller's responsibility.
func (c *Context) Remove(task *Task) error {
	for i, t := range c.Tasks {
		if t == task {
			c.Tasks = append(c.Tasks[:i], c.Tasks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("could not find task %v in list", task.Raw)
}

func (c *Context) Sort() *Context {
	slices.SortFunc(c.Tasks, func(i, j *Task) int {
		var a string = i.Priority
		var b string = j.Priority
		if a == "" {
			a = "Z"
		}
		if b == "" {
			b = "Z"
		}
		return strings.Compare(a, b)
	})
	return c
}

func (c *Context) RemoveByStrId(strid string) error {
	t, err := c.GetTaskByStrId(strid)
	if err != nil {
		return err
	}
	return c.Remove(t)
}

// Filepath returns the on-disk location of this context under the viper-
// configured data directory. It performs no I/O — purely a path builder.
// Kept for the two $EDITOR shell-out commands (edit-done, TUI EditFile)
// that need a path to hand to an external process.
//
// TODO(refactor): a future step should move path resolution onto the
// storage layer so domain stops depending on viper at all.
func (c *Context) Filepath() string {
	return path.Join(vars.Get(vars.ConfigKeyDataDir), c.Name+vars.DefaultFileExtension)
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

func (c *Context) Replace(old *Task, new *Task) error {
	index, err := c.GetTaskIndex(old)
	if err != nil {
		return err
	}
	c.Tasks[index] = new
	return nil
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

func (c *Context) Show() {
	if len(c.Tasks) == 0 {
		fmt.Printf("(%s): Context is empty.\n", c.ConsoleString())
		return
	}
	showTasks(c.Tasks)
	fmt.Printf("(%s): %d tasks\n", c.ConsoleString(), len(c.Tasks))
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
