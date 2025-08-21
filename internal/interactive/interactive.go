package interactive

import (
	"log"
	"slices"

	"github.com/bricef/htt/internal/todo"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	context       *todo.Context
	cursor        int // which to-do list item our cursor is pointing at
	contexts      []*todo.Context
	contextCursor int
	width         int
	height        int
	keys          KeyBindingController
}

var controller = NewKeyBindingController().
	AddShortBinding(Help, key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	)).
	AddShortBinding(Quit, key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q", "quit"),
	)).
	AddBinding(Do, key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "do"),
	)).
	AddBinding(Up, key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	)).
	AddBinding(Down, key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	)).
	AddBinding(NextContext, key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "move right"),
	)).
	AddBinding(PreviousContext, key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "move left"),
	))

func Model(ctx *todo.Context) model {
	contexts := todo.GetContexts()
	contexts = append(contexts, todo.NewContext("done"))
	contexts = slices.Insert(contexts, 0, todo.NewContext("todo"))

	selected := slices.IndexFunc(contexts, func(c *todo.Context) bool {
		return c.Equals(ctx)
	})

	return model{
		context:       ctx,
		cursor:        0,
		contexts:      contexts,
		contextCursor: selected,
		keys:          controller,
	}
}

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	// Is it a key press?
	case tea.KeyMsg:
		action := m.keys.GetAction(msg)
		log.Printf("action: %s", action.description)
		return action.Act(m)
	}

	return m, nil
}

func selected(l lipgloss.Style) lipgloss.Style {
	return l.
		Foreground(lipgloss.Color("#F45634")).
		Background(lipgloss.Color("#FFFFFF")).
		Bold(true)
}

func (m model) View() string {
	menuitem := lipgloss.NewStyle().Padding(0, 2).Align(lipgloss.Center)
	done := lipgloss.NewStyle().
		Inherit(menuitem).
		Padding(0, 2).
		Foreground(lipgloss.Color("#00FF00"))

	menustr := ""
	for i, context := range m.contexts {
		style := menuitem
		if context.Name == "done" {
			style = done
		}
		if i == m.contextCursor {
			style = selected(style)
		}

		menustr += style.Render(context.Name)

	}

	title := lipgloss.NewStyle().
		Align(lipgloss.Left).
		Foreground(lipgloss.Color("#00FF00")).
		Width(m.width / 2).
		Render("htt")

	menu := lipgloss.NewStyle().
		Align(lipgloss.Right).
		Width(m.width / 2).
		Render(menustr)

	header := lipgloss.NewStyle().
		Align(lipgloss.Right).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		Width(m.width).
		Render(title + menu)

	footer := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(m.width).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		Render("Press q to quit.")

	content := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height - lipgloss.Height(header) - lipgloss.Height(footer)).
		Align(lipgloss.Left)

	s := ""

	if m.context.Name == "done" {
		s += RenderDoneList(m.context.Tasks, m.cursor)
	} else {
		s += RenderTaskList(m.context.Tasks, m.cursor)
	}

	// Send the UI for rendering
	return lipgloss.JoinVertical(lipgloss.Top, header, content.Render(s), footer)
}
