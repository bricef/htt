package interactive

import (
	"log"
	"slices"

	"github.com/bricef/htt/internal/todo"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
<<<<<<< HEAD
	"github.com/charmbracelet/lipgloss"
||||||| parent of cd00e7d (Enable task interaction in interactive mode)
	"github.com/charmbracelet/lipgloss/v2"

=======
	"github.com/charmbracelet/lipgloss"

>>>>>>> cd00e7d (Enable task interaction in interactive mode)
	"github.com/lucasb-eyer/go-colorful"
)

// {
// 	'gunmetal': {
// 		DEFAULT: '#1f363d',
// 		100: '#060b0c',
// 		200: '#0c1618', 300: '#122025', 400: '#192b31', 500: '#1f363d', 600: '#3b6775', 700: '#5898ab', 800: '#90bac7', 900: '#c7dde3'
// 	},
// 	'cerulean': {
// 		DEFAULT: '#40798c', 100: '#0d181c', 200: '#1a3038', 300: '#274954', 400: '#336170', 500: '#40798c', 600: '#579bb2', 700: '#81b4c5',
// 		800: '#abcdd8', 900: '#d5e6ec'
// 	},
// 	'verdigris': {
// 		DEFAULT: '#70a9a1', 100: '#152321', 200: '#2a4642', 300: '#3f6964', 400: '#548c85',
// 		500: '#70a9a1', 600: '#8cbab4', 700: '#a9cbc7', 800: '#c6ddda', 900: '#e2eeec'
// 	},
// 	'cambridge_blue': {
// 		DEFAULT: '#9ec1a3', 100: '#1b2b1e', 200: '#37563c', 300: '#528159', 400: '#74a67b',
// 		500: '#9ec1a3', 600: '#b2ceb6', 700: '#c5dac8', 800: '#d8e6db', 900: '#ecf3ed'
// 	},
// 	'tea_green': {
// 		DEFAULT: '#cfe0c3', 100: '#28371c', 200: '#4f6e39', 300: '#77a655', 400: '#a3c38b',
// 		500: '#cfe0c3', 600: '#d8e6cf', 700: '#e2ecdb', 800: '#ecf3e7', 900: '#f5f9f3'
// 	}
// }

const (
	Gunmetal      = "#1f363d"
	Cerulean      = "#40798c"
	Verdigris     = "#70a9a1"
	CambridgeBlue = "#9ec1a3"
	TeaGreen      = "#cfe0c3"
)

const (
	background_color = lipgloss.Color("#192b31")
	foreground_color = lipgloss.Color(TeaGreen)
	selected_color   = lipgloss.Color(Cerulean)
	cursor_color     = lipgloss.Color(Verdigris)
)

func saturation(color lipgloss.Color, s float64) lipgloss.Color {
	c, _ := colorful.Hex(string(color))
	h, l, _ := c.HSLuv()
	c = colorful.HSLuv(h, s, l)
	return lipgloss.Color(c.Hex())
}

func luminance(color lipgloss.Color, l float64) lipgloss.Color {
	c, _ := colorful.Hex(string(color))
	h, _, s := c.HSLuv()
	c = colorful.HSLuv(h, s, l)
	return lipgloss.Color(c.Hex())
}

var color_subtle = lipgloss.AdaptiveColor{
	Light: "#909090",
	Dark:  "#626262",
}
var color_subtle_separator = lipgloss.AdaptiveColor{
	Light: "#DDDADA",
	Dark:  "#3C3C3C",
}
var color_subtle_desc = lipgloss.AdaptiveColor{
	Light: "#B2B2B2",
	Dark:  "#4A4A4A",
}

type model struct {
	context       *todo.Context
	cursor        int // which to-do list item our cursor is pointing at
	contexts      []*todo.Context
	contextCursor int
	width         int
	height        int
	keys          KeyBindingController
	showHelp      bool
	newTask       bool
	textInput     textinput.Model
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
	AddShortBinding(CommandMode, key.NewBinding(
		key.WithKeys(":"),
		key.WithHelp(":", "command mode"),
	)).
	AddBinding(Do, key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "do"),
	)).
	AddBinding(Delete, key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
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
	)).
	AddBinding(EditFile, key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "edit file"),
	)).
	AddBinding(NewTask, key.NewBinding(
		key.WithKeys("n", "a"),
		key.WithHelp("n/a", "new task"),
	)).
	AddBinding(IncreasePriority, key.NewBinding(
		key.WithKeys("+"),
		key.WithHelp("+", "increase priority"),
	)).
	AddBinding(DecreasePriority, key.NewBinding(
		key.WithKeys("-"),
		key.WithHelp("-", "decrease priority"),
	))

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

	ti := textinput.New()
	ti.Placeholder = "(A) Do a thing for +project in @context"

	ti.Prompt = "New Task > "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(cursor_color).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(background_color).Background(foreground_color)
	ti.PlaceholderStyle = ti.TextStyle.Foreground(color_subtle)
	ti.Width = 100

	return model{
		context:       ctx,
		cursor:        0,
		contexts:      contexts,
		contextCursor: selected,
		keys:          controller,
		newTask:       false,
		textInput:     ti,
	}
}

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	// Is it a key press?
	case tea.KeyMsg:
		if m.textInput.Focused() {
			switch msg.Type {
			case tea.KeyEnter:
				return AddTask.Act(m)
			case tea.KeyEsc:
				return CancelNewTask.Act(m)
			case tea.KeyCtrlC:
				return m, tea.Quit
			}
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		} else {
			action := m.keys.GetAction(msg)
			log.Printf("action: %s", action.description)
			return action.Act(m)
		}
	}

	return m, cmd
}

func selected(l lipgloss.Style) lipgloss.Style {
	return l.
		Foreground(foreground_color).
		Background(selected_color).
		Bold(true)
}

var keyStyle = lipgloss.NewStyle().Foreground(color_subtle).Bold(true)
var descStyle = lipgloss.NewStyle().Foreground(color_subtle_desc)
var sepStyle = lipgloss.NewStyle().Foreground(color_subtle_separator)

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

	footer := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(m.width).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(color_subtle).
		Render(helpMenu.View(m.keys))

	addWidget := ""
	if m.newTask {
		addWidget = m.textInput.View()
	}

	content := baseStyle.
		Width(m.width).
		Height(m.height - lipgloss.Height(header) - lipgloss.Height(addWidget) - lipgloss.Height(footer)).
		Align(lipgloss.Left)

	s := ""

	if m.context.Name == "done" {
		s += RenderDoneList(m.context.Tasks, m.cursor)
	} else {
		s += RenderTaskList(m.context.Tasks, m.cursor)
	}

	app := lipgloss.JoinVertical(
		lipgloss.Top,
		header,
		content.Render(s),
		addWidget,
		footer,
	)

	return app
}
