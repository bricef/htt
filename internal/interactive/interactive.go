package interactive

import (
	"log"
	"slices"

	"github.com/bricef/htt/internal/todo"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	lipgloss "github.com/charmbracelet/lipgloss"
)

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type model struct {
	context       *todo.Context
	cursor        int // which to-do list item our cursor is pointing at
	contexts      []*todo.Context
	contextCursor int
	width         int
	height        int
	keys          KeyBindingController
	showHelp      bool
	prompt        Prompt
	list          TaskList
	focused       tea.Model
}

var controller = NewKeyBindingController().
	// AddShortBinding(Help, key.NewBinding(
	// 	key.WithKeys("?"),
	// 	key.WithHelp("?", "toggle help"),
	// )).
	AddShortBinding(Quit, key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q", "quit"),
	)).
	// AddShortBinding(CommandMode, key.NewBinding(
	// 	key.WithKeys(":"),
	// 	key.WithHelp(":", "command mode"),
	// )).
	// AddBinding(Do, key.NewBinding(
	// 	key.WithKeys("x"),
	// 	key.WithHelp("x", "do"),
	// )).
	// AddBinding(Delete, key.NewBinding(
	// 	key.WithKeys("d"),
	// 	key.WithHelp("d", "delete"),
	// )).
	// AddBinding(Up, key.NewBinding(
	// 	key.WithKeys("up", "k"),
	// 	key.WithHelp("↑/k", "move up"),
	// )).
	// AddBinding(Down, key.NewBinding(
	// 	key.WithKeys("down", "j"),
	// 	key.WithHelp("↓/j", "move down"),
	// )).
	AddBinding(NextContext, key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "move right"),
	)).
	AddBinding(PreviousContext, key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "move left"),
	))
	// AddBinding(EditFile, key.NewBinding(
	// 	key.WithKeys("f"),
	// 	key.WithHelp("f", "edit file"),
	// )).
	// AddBinding(NewTask, key.NewBinding(
	// 	key.WithKeys("n", "a"),
	// 	key.WithHelp("n/a", "new task"),
	// )).
	// AddBinding(IncreasePriority, key.NewBinding(
	// 	key.WithKeys("+"),
	// 	key.WithHelp("+", "increase priority"),
	// )).
	// AddBinding(DecreasePriority, key.NewBinding(
	// 	key.WithKeys("-"),
	// 	key.WithHelp("-", "decrease priority"),
	// ))

func Model(ctx *todo.Context) model {
	contexts := todo.GetContexts()
	contexts = append(contexts, todo.NewContext("done"))
	// todoIndex := slices.IndexFunc(contexts, func(c *todo.Context) bool {
	// 	return c.Name == "todo"
	// })
	// if todoIndex == -1 {
	// 	contexts = slices.Insert(contexts, 0, todo.NewContext("todo"))
	// }

	selected := slices.IndexFunc(contexts, func(c *todo.Context) bool {
		return c.Equals(ctx)
	})

	list := NewTaskList(ctx)
	return model{
		context:       ctx,
		cursor:        0,
		contexts:      contexts,
		contextCursor: selected,
		keys:          controller,
		prompt:        NewPrompt(),
		list:          list,
		focused:       &list,
	}
}

func (m model) Focus(f tea.Model) {
	m.focused = f
}

func (m model) isFocused(f tea.Model) bool {
	return m.focused == f
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {

	case AddedTaskMsg:
		m.context.Add(msg.task)
		m.context.Sync()
		m.list = NewTaskList(m.context)
		// Update focused to point to the new list if it was pointing to the old one
		if _, ok := m.focused.(*TaskList); ok {
			m.focused = &m.list
		}
		cmds = append(cmds, tea.ClearScreen)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	// Is it a key press?
	case tea.KeyMsg:
		if m.focused == nil {
			log.Printf("Default model update")
			action := m.keys.GetAction(msg)
			log.Printf("action: %s", action.description)
			var modelUpdate tea.Model
			modelUpdate, cmd = action.Act(m)
			m = modelUpdate.(model)
			cmds = append(cmds, cmd)
		} else {
			log.Printf("Focused model update")
			m.focused, cmd = m.focused.Update(msg)
			// If focused is the TaskList, keep m.list in sync
			if focusedList, ok := m.focused.(*TaskList); ok {
				m.list = *focusedList
				m.focused = &m.list
			} else if focusedList, ok := m.focused.(TaskList); ok {
				// Handle case where Update returns a value, not a pointer
				m.list = focusedList
				m.focused = &m.list
			}
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	baseStyle := lipgloss.NewStyle()

	menuitem := baseStyle.Padding(0, 2).Align(lipgloss.Center)

	done := baseStyle.
		Inherit(menuitem).
		Padding(0, 2).
		Background(foreground_color).
		Foreground(background_color)

	menustr := ""
	for i, context := range m.contexts {
		itemStyle := menuitem
		if context.Name == "done" {
			itemStyle = done
		}
		if i == m.contextCursor {
			itemStyle = selected(itemStyle)
		}

		menustr += itemStyle.Render(context.Name)

	}

	title := baseStyle.
		Align(lipgloss.Left).
		Foreground(foreground_color).
		Width(m.width / 2).
		Render("htt")

	menu := baseStyle.
		Align(lipgloss.Right).
		Width(m.width / 2).
		Render(menustr)

	header := baseStyle.
		Align(lipgloss.Right).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(color_subtle).
		Width(m.width).
		Render(title + menu)

	helpMenu := help.New()
	helpMenu.ShowAll = m.showHelp
	helpMenu.Styles = help.Styles{
		ShortKey:       keyStyle,
		ShortDesc:      descStyle,
		ShortSeparator: sepStyle,
		Ellipsis:       sepStyle,
		FullKey:        keyStyle,
		FullDesc:       descStyle,
		FullSeparator:  sepStyle,
	}

	// footer := lipgloss.NewStyle().
	// 	Align(lipgloss.Center).
	// 	Width(m.width).
	// 	Border(lipgloss.NormalBorder(), true, false, false, false).
	// 	BorderForeground(color_subtle).
	// 	Render(helpMenu.View(m.keys))

	addWidget := ""
	if m.isFocused(m.prompt) {
		addWidget = m.prompt.View()
	}

	contentHeight :=
		m.height -
			lipgloss.Height(header) -
			lipgloss.Height(addWidget)
		// lipgloss.Height(footer)

	// content := baseStyle.
	// 	Width(m.width).
	// 	Height(contentHeight).
	// 	Align(lipgloss.Left)

	// s := ""

	// if m.context.Name == "done" {
	// 	s += RenderDoneList(m.context.Tasks, m.cursor)
	// } else {
	// 	s += RenderTaskList(m.context.Tasks, m.cursor)
	// }

	m.list.list.SetWidth(m.width)
	m.list.list.SetHeight(contentHeight)
	app := lipgloss.JoinVertical(
		lipgloss.Top,
		header,
		// content.Render(s),
		m.list.View(),
		addWidget,
		// footer,
	)

	return app
}
