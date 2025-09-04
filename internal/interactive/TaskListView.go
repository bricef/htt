package interactive

import (
	"fmt"

	"github.com/bricef/htt/internal/todo"
	"github.com/charmbracelet/lipgloss"
)

const CURSOR = ">"

var currentTask = lipgloss.NewStyle().
	Foreground(foreground_color).
	Background(selected_color)

func RenderTask(task *todo.Task, selected bool) string {
	if selected {
		return currentTask.Render(task.RawString())
	}
	return task.ConsoleString()
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
