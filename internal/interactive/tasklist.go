package interactive

import (
	"github.com/bricef/htt/internal/todo"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type TaskList struct {
	model list.Model
}

func (t TaskList) Init() tea.Cmd {
	return nil
}
func (t TaskList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := t.model.Update(msg)
	return TaskList{model: m}, cmd
}
func (t TaskList) View() string {
	return t.model.View()
}

func NewTaskList(ctx *todo.Context) TaskList {
	items := []list.Item{}
	for _, task := range ctx.Tasks {
		items = append(items, item{title: task.Raw, desc: ""})
	}

	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	d.SetSpacing(0)
	return TaskList{model: list.New(items, d, 100, 40)}
}
