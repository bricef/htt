package interactive

import (
	"log"

	"github.com/bricef/htt/internal/todo"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type InnerItem struct {
	task *todo.Task
}

func (i InnerItem) FilterValue() string {
	return i.task.Raw
}

type ListItem struct {
	Task *todo.Task
	Item InnerItem
}

func (i ListItem) Title() string {
	return i.Task.Raw
}
func (i ListItem) Description() string {
	return ""
}
func (i ListItem) FilterValue() string {
	return i.Task.Raw
}

func NewListItem(task *todo.Task) ListItem {
	return ListItem{
		Task: task,
		Item: InnerItem{task: task},
	}
}

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
		items = append(items, NewListItem(task))
	}
	// need to set up custom delegate and additional list actions

	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	d.SetSpacing(0)
	// width and height will be adjusted on update
	return TaskList{list: list.New(items, d, 10, 10)}
}
