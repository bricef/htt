package interactive

import (
	"fmt"

	"github.com/bricef/htt/internal/todo"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const CURSOR = ">"

type TaskList struct {
	Height int
	Width  int
	done   bool
	tasks  []*todo.Task
	cursor int
}

func NewTaskList(tasks []*todo.Task, done bool) *TaskList {
	return &TaskList{
		done:   done,
		tasks:  tasks,
		cursor: 0,
	}
}

func (t *TaskList) Init() tea.Cmd {
	return nil
}

func (t *TaskList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return t, nil
}

func (t *TaskList) View() string {
	if t.done {
		return RenderDoneList(t.tasks, t.cursor)
	}
	return RenderTaskList(t.tasks, t.cursor)
}

var currentTask = lipgloss.NewStyle().
	Foreground(foreground_color).
	Background(selected_color)

func RenderTask(task *todo.Task, selected bool) string {
	if selected {
		return currentTask.Render(task.RawString())
	}
	return task.RawString()
}

func RenderTaskList(tasks []*todo.Task, cursor int) string {
	s := ""
	// Iterate over our choices
	for i, choice := range tasks {

		// Is the cursor pointing at this choice?
		cursorChar := " " // no cursor
		if cursor == i {
			cursorChar = CURSOR // cursor!
		}

		// Render the row
		s += fmt.Sprintf("%s %4d %s\n", cursorChar, i, RenderTask(choice, cursor == i))
	}
	return s
}

func RenderDoneList(tasks []*todo.Task, cursor int) string {
	s := ""
	// Iterate over our choices
	for i, choice := range tasks {

		// Is the cursor pointing at this choice?
		cursorChar := " " // no cursor
		if cursor == i {
			cursorChar = CURSOR // cursor!
		}

		// Render the row
		s += fmt.Sprintf("%s %s\n", cursorChar, RenderTask(choice, cursor == i))
	}
	return s
}
