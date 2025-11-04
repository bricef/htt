package interactive

import (
	"log"

	"github.com/bricef/htt/internal/todo"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type TaskList struct {
	list list.Model
}

func (t TaskList) Init() tea.Cmd {
	return nil
}
func (t TaskList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	log.Printf("TaskList list: %v", t.list.Cursor())
	t.list, cmd = t.list.Update(msg)

	log.Printf("TaskList list: %v", t.list.Cursor())
	return t, cmd
}
func (t TaskList) View() string {
	return t.list.View()
}

func NewTaskList(ctx *todo.Context) TaskList {
	items := []list.Item{}
	for _, task := range ctx.Tasks {
		items = append(items, item{title: task.Raw, desc: ""})
	}

	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	d.SetSpacing(0)
	return TaskList{list: list.New(items, d, 100, 40)}
}
