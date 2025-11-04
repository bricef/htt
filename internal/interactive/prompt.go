package interactive

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Prompt struct {
	ti textinput.Model
}

func NewPrompt() Prompt {

	ti := textinput.New()
	ti.Placeholder = "(A) Do a thing for +project in @context"
	ti.Prompt = "New Task > "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(cursor_color).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(background_color).Background(foreground_color)
	ti.PlaceholderStyle = ti.TextStyle.Foreground(color_subtle)
	ti.Width = 100

	return Prompt{
		ti: ti,
	}
}

func (p Prompt) Focus() {
	p.ti.Focus()
	p.ti.Cursor.Blink = true
}

func (p Prompt) GetValue() string {
	return p.ti.Value()
}

func (p Prompt) Reset() {
	p.ti.SetValue("")
	p.ti.Blur()
}

func (p Prompt) Init() tea.Cmd {
	return nil
}

func (p Prompt) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			t := p.GetValue()
			p.Reset()
			if t == "" {
				return p, nil
			}
			return p, AddTask(t)
		case tea.KeyEsc:
			p.Reset()
			return p, nil
		case tea.KeyCtrlC:
			return p, tea.Quit
		}
	}
	return p, nil
}

func (p Prompt) View() string {
	return p.ti.View()
}
