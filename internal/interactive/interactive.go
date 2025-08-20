package interactive

import (
	"slices"
	"time"

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
}

type Binding struct {
	action  Action
	binding key.Binding
}

type ActionController []Binding

func NewActionController() ActionController {
	return make(ActionController, 0)
}

func (c ActionController) addBinding(action Action, binding key.Binding) ActionController {
	return append(c, Binding{action, binding})
}

func Model(ctx *todo.Context) helpMode {
	contexts := todo.GetContexts()
	contexts = append(contexts, todo.NewContext("done"))
	contexts = slices.Insert(contexts, 0, todo.NewContext("todo"))

	selected := slices.IndexFunc(contexts, func(c *todo.Context) bool {
		return c.Equals(ctx)
	})

	return helpMode{
		context:       ctx,
		cursor:        0,
		contexts:      contexts,
		contextCursor: selected,
	}
}

func (m helpMode) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

func (m helpMode) changeContext() {
	name := m.contexts[m.contextCursor].Name
	if name != "done" {
		todo.SetCurrentContext(name)
	}
	m.context = todo.NewContext(name)
	m.context.Read()
}

func (m helpMode) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit

		// The "up" and "k" keys move the cursor up
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			if m.cursor < len(m.context.Tasks)-1 {
				m.cursor++
			}

		case "ctrl+t", "right", "l":
			if m.contextCursor < len(m.contexts)-1 {
				m.contextCursor++
				name := m.contexts[m.contextCursor].Name
				if name != "done" {
					todo.SetCurrentContext(name)
				}
				m.context = todo.NewContext(name)
				m.context.Read()
				m.cursor = 0
			}

		case "ctrl+shift+t", "left", "h":
			if m.contextCursor > 0 {
				m.contextCursor--
				name := m.contexts[m.contextCursor].Name
				if name != "done" {
					todo.SetCurrentContext(name)
				}
				m.context = todo.NewContext(name)
				m.context.Read()
				m.cursor = 0
			}

		case "x":
			// TODO: Log errors if any
			t, err := m.context.GetTaskById(m.cursor)
			if err != nil {
				return m, tea.Quit
			}
			t.Do(m.context, time.Now())
			err = todo.Move(t, m.context, todo.NewContext("done"))
			if err != nil {
				return m, tea.Quit
			}
			err = m.context.Sync()
			if err != nil {
				return m, tea.Quit
			}
		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func selected(l lipgloss.Style) lipgloss.Style {
	return l.
		Foreground(lipgloss.Color("#F45634")).
		Background(lipgloss.Color("#FFFFFF")).
		Bold(true)
}

type keyMap struct {
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding
	Help  key.Binding
	Quit  key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right}, // first column
		{k.Help, k.Quit},                // second column
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "move left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "move right"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

func (m helpMode) View() string {

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
