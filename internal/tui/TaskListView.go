package tui

import (
	"fmt"

	"github.com/bricef/htt/internal/domain"
	"github.com/charmbracelet/lipgloss"
)

const CURSOR = ">"

// RenderTask renders one task, highlighted with the selected style when
// the cursor is on it.
func RenderTask(task *domain.Task, isSelected bool, selected lipgloss.Style) string {
	if isSelected {
		return selected.Render(task.RawString())
	}
	return task.RawString()
}

func RenderTaskList(tasks []*domain.Task, cursor int, selected lipgloss.Style) string {
	s := ""
	// Iterate over our choices
	for i, choice := range tasks {

		// Is the cursor pointing at this choice?
		cursorChar := " " // no cursor
		if cursor == i {
			cursorChar = CURSOR // cursor!
		}

		// Render the row
		s += fmt.Sprintf("%s %4d %s\n", cursorChar, i, RenderTask(choice, cursor == i, selected))
	}
	return s
}

func RenderDoneList(tasks []*domain.Task, cursor int, selected lipgloss.Style) string {
	s := ""
	// Iterate over our choices
	for i, choice := range tasks {

		// Is the cursor pointing at this choice?
		cursorChar := " " // no cursor
		if cursor == i {
			cursorChar = CURSOR // cursor!
		}

		// Render the row
		s += fmt.Sprintf("%s %s\n", cursorChar, RenderTask(choice, cursor == i, selected))
	}
	return s
}
