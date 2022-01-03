package todo

import (
	"fmt"
	"os"
	"path"
	"strconv"

	"github.com/bricef/htt/internal/utils"
	"github.com/bricef/htt/internal/vars"
	"github.com/fatih/color"
)

type Context struct {
	Name  string
	Tasks []*Task
}

func NewContext(name string) *Context {
	return &Context{
		Name:  name,
		Tasks: []*Task{},
	}
}

func (c *Context) Add(t *Task) *Context {
	c.Tasks = append(c.Tasks, t)
	return c
}

func (c *Context) Remove(task *Task) error {
	for i, t := range c.Tasks {
		if t == task {
			c.Tasks = append(c.Tasks[:i], c.Tasks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("Could not find task %v in list.", task.Raw)
}

func (c *Context) RemoveByStrId(strid string) error {
	t, err := c.GetTaskByStrId(strid)
	if err != nil {
		return err
	}

	return c.Remove(t)
}

func (c *Context) Read() *Context {
	lines := utils.ReadLines(c.Filepath())
	for i, line := range lines {
		if line != "" {
			t := NewTask(line)
			t.Line = i
			c.Add(t)
		}
	}
	return c
}

func (c *Context) Filepath() string {
	return path.Join(vars.Get(vars.ConfigKeyDataDir), c.Name+vars.DefaultFileExtension)
}

func (c *Context) File() *os.File {
	contextPath := c.Filepath()
	utils.EnsurePath(contextPath)
	f, err := os.OpenFile(contextPath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	utils.DieOnError("Could not open file: "+contextPath+": ", err)
	return f
}

func (c *Context) Sync() error {
	originalPath := c.Filepath()
	backupPath := c.Filepath() + ".bak"
	err := os.Rename(originalPath, backupPath)
	if err != nil {
		return err //utils.DieOnError("Could not create a backup file.", err)
	}

	f := c.File()
	defer f.Close()
	for _, task := range c.Tasks {
		_, err := f.WriteString(task.String() + "\n")
		if err != nil {
			return err //utils.DieOnError("Failed to write todo to file", err)
		}
	}
	os.Remove(backupPath)
	return nil
}

func (c *Context) GetTaskById(index int) (*Task, error) {
	if len(c.Tasks) == 0 {
		return nil, fmt.Errorf("Task list was empty")
	}
	if index > len(c.Tasks) || index < 0 {
		return nil, fmt.Errorf("Item selected is outside of range")
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
	return -1, fmt.Errorf("Could not find task in context.")
}

func (c *Context) Replace(old *Task, new *Task) error {
	index, err := c.GetTaskIndex(old)
	if err != nil {
		return err
	}
	c.Tasks[index] = new
	return nil
}

func (c *Context) String() string {
	if vars.GetBool(vars.ConfigKeyDisableColor) {
		return c.Name
	} else {
		return color.GreenString(c.Name)
	}
}

func showTasks(ts []*Task) {
	fmt.Println()
	for _, todo := range ts {
		fmt.Printf("%3d %s\n", todo.Line, todo.String())
	}
	fmt.Println()

}

func (c *Context) Show() {
	if len(c.Tasks) == 0 {
		fmt.Printf("(%s): Context is empty.\n", c.String())
		return
	}
	showTasks(c.Tasks)
	fmt.Printf("(%s): %d tasks\n", c.String(), len(c.Tasks))

}

func (c *Context) ShowOnly(predicate func(*Task) bool) {
	ts := c.Search(predicate)

	if len(ts) == 0 {
		fmt.Printf("(%s): No tasks matched query.\n", c.String())
		return
	}

	showTasks(ts)

	fmt.Printf("(%s): %d out of %v tasks matched query.\n", c.String(), len(ts), len(c.Tasks))
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
