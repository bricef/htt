package interactive

import (
	"bytes"
	"io"
	"log"

	"github.com/bricef/htt/internal/todo"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

type TodoItemDelegate struct {
	list.DefaultDelegate
}

func RenderTask(task *todo.Task) string {

	base := lipgloss.NewStyle().PaddingLeft(2)
	A := base.Foreground(lipgloss.Color("#ff0202"))
	B := base.Foreground(lipgloss.Color("#08c600"))
	C := base.Foreground(lipgloss.Color("#f0c546"))

	switch task.Priority {
	case "A":
		return A.Render(task.Raw)
	case "B":
		return B.Render(task.Raw)
	case "C":
		return C.Render(task.Raw)
	default:
		return base.Render(task.Raw)
	}
}

func (d TodoItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {

	isSelected := index == m.Cursor()

	cursorStyle := lipgloss.NewStyle().
		Foreground(cursor_color).
		Bold(true)

	selectedStyle := lipgloss.NewStyle().
		Underline(true).
		Foreground(cursor_color).
		PaddingLeft(2)

	// Use a buffer to capture the default rendering
	var buf bytes.Buffer

	if isSelected {
		buf.WriteString(cursorStyle.Render(" ►"))
	} else {
		buf.WriteString("  ")
	}

	if isSelected {
		buf.WriteString(selectedStyle.Render(item.(ListItem).Task.Raw))
	} else {
		buf.WriteString(RenderTask(item.(ListItem).Task))
	}

	io.WriteString(w, buf.String())
}

var IncreasePriorityBinding = key.NewBinding(
	key.WithKeys("Y", "y"),
	key.WithHelp("Y/y", "Increase priority"),
)

var DecreasePriorityBinding = key.NewBinding(
	key.WithKeys("N"),
	key.WithHelp("N", "Decrease priority"),
)

func (d TodoItemDelegate) ShortHelp() []key.Binding {
	return []key.Binding{
		IncreasePriorityBinding,
		DecreasePriorityBinding,
	}
}

func (d TodoItemDelegate) FullHelp() [][]key.Binding {
	return [][]key.Binding{}
}

func (d TodoItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Printf("TodoItemDelegate Update: %v", msg)
		if key.Matches(msg, IncreasePriorityBinding) {
			log.Printf("Match: %v", msg)
		}
	}
	return nil
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

	d := TodoItemDelegate{
		DefaultDelegate: list.NewDefaultDelegate(),
	}
	d.ShowDescription = false
	d.SetSpacing(0)

	l := list.New(items, d, 10, 10)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)

	styles := list.DefaultStyles()
	l.Styles = styles

	// width and height will be adjusted on update
	return TaskList{list: l}
}
