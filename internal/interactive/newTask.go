package interactive

import (
	tea "github.com/charmbracelet/bubbletea"
)

type TaskMode struct {
	task string
}

func NewTaskMode() TaskMode {
	return TaskMode{
		task: "",
	}
}

func (m TaskMode) Init() tea.Cmd {
	return nil
}

func (m TaskMode) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	return m, nil
}

func (m TaskMode) View() string {
	return m.task
}
