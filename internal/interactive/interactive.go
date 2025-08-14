package interactive

import (
	"fmt"
	"time"

	"github.com/bricef/htt/internal/todo"
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

func Model(ctx *todo.Context) model {
	return model{
		context:       ctx,
		cursor:        0,
		contexts:      todo.GetContexts(),
		contextCursor: 0,
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
			// change context
			if m.contextCursor < len(m.contexts)-1 {
				m.contextCursor++
				m.context = todo.NewContext(m.contexts[m.contextCursor].Name)
				m.context.Read()
			}

		case "ctrl+shift+t", "left", "h":
			// change context
			if m.contextCursor > 0 {
				m.contextCursor--
				m.context = todo.NewContext(m.contexts[m.contextCursor].Name)
				m.context.Read()
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

func (m model) View() string {

	menuitem := lipgloss.NewStyle().Padding(0, 2).Align(lipgloss.Center)
	selected := lipgloss.NewStyle().
		Inherit(menuitem).
		Padding(0, 2).
		Foreground(lipgloss.Color("#F45634")).
		Background(lipgloss.Color("#FFFFFF")).
		Bold(true)

	menustr := ""
	for i, context := range m.contexts {
		if i == m.contextCursor {
			menustr += selected.Render(context.Name)
		} else {
			menustr += menuitem.Render(context.Name)
		}
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
		Height(m.height-lipgloss.Height(header)-lipgloss.Height(footer)).
		Align(lipgloss.Left, lipgloss.Center)

	s := ""

	// Iterate over our choices
	for i, choice := range m.context.Tasks {

		// Is the cursor pointing at this choice?
		cursor := " " // no cursor
		if m.cursor == i {
			cursor = ">" // cursor!
		}

		// Render the row
		s += fmt.Sprintf("%s [ ] %s\n", cursor, choice.ConsoleString())
	}

	// Send the UI for rendering
	return lipgloss.JoinVertical(lipgloss.Top, header, content.Render(s), footer)

}
