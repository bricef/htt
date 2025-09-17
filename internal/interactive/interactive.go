package interactive

import (
	"slices"

	"github.com/bricef/htt/internal/todo"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	lipgloss "github.com/charmbracelet/lipgloss"
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

type app struct {
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
	list          list.Model
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

func App() app {
	ctx := todo.GetCurrentContext() // will return default or exit.
	contexts := todo.GetContexts()

	// Only append if it's not already in the list
	if !slices.ContainsFunc(contexts, func(c *todo.Context) bool {
		return c.Name == "done"
	}) {
		contexts = append(contexts, todo.NewContext("done"))
	}

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

	list := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	list.SetItems(toItems(ctx.Tasks))
	list.Title = ctx.Name
	return app{
		context:       ctx,
		cursor:        0,
		contexts:      contexts,
		contextCursor: selected,
		keys:          controller,
		newTask:       false,
		textInput:     ti,
		list:          list,
	}
}

func (m app) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

type item struct {
	title, description string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.description }
func (i item) FilterValue() string { return i.title }

func toItems(tasks []*todo.Task) []list.Item {
	items := []list.Item{}
	for _, task := range tasks {
		items = append(items, item{title: task.RawString(), description: ""})
	}

	return items
}

func (m app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		for child := range m.getChildren() {
			child.Update(msg)
			cmds = append(cmds, cmd)
		}

	// Is it a key press?
	case tea.KeyMsg:

		// if m.textInput.Focused() {
		// 	switch msg.Type {
		// 	case tea.KeyEnter:
		// 		return AddTask.Act(m)
		// 	case tea.KeyEsc:
		// 		return CancelNewTask.Act(m)
		// 	case tea.KeyCtrlC:
		// 		return m, tea.Quit
		// 	}
		// 	m.textInput, cmd = m.textInput.Update(msg)
		// 	cmds = append(cmds, cmd)
		// } else {
		child := m.getFocusedChild()
		if child != nil {
			child.Update(msg)
			cmds = append(cmds, cmd)
		} else {
			m.keys, cmd = m.keys.Update(msg)
			cmds = append(cmds, cmd)
		}
		// m.keys, cmd = m.keys.Update(msg)
		// cmds = append(cmds, cmd)

		// m.list, cmd = m.list.Update(msg)
		// cmds = append(cmds, cmd)

		// action := m.keys.GetAction(msg)
		// log.Printf("action cmd: %s %v", action.description, cmd)
		// newModel, cmd := action.Act(m)
		// if cmd != nil {
		// 	m = newModel.(app)
		// 	cmds = append(cmds, cmd)
		// } else {
		// 	m.list, cmd = m.list.Update(msg)
		// 	log.Printf("list cmd: %v %v", msg, cmd)
		// 	if cmd != nil {
		// 		log.Printf("list cmd: %v %v", msg, cmd)
		// 		cmds = append(cmds, cmd)
		// 	}
		// }

		// }
	}

	return m, tea.Sequence(cmds...)
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

func (m app) View() string {
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

	_ = lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(m.width).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(color_subtle).
		Render(helpMenu.View(m.keys))

	addWidget := ""
	if m.newTask {
		addWidget = lipgloss.NewStyle().
			Padding(1, 0).
			Width(m.width).
			Render(m.textInput.View())
	}

	contentHeight := m.height - lipgloss.Height(header) - lipgloss.Height(addWidget) // - lipgloss.Height(footer)
	content := baseStyle.
		Width(m.width).
		Height(contentHeight).
		Align(lipgloss.Left)

	m.list.SetSize(m.width, contentHeight)

	s := m.list.View()

	// if m.context.Name == "done" {
	// 	s += RenderDoneList(m.context.Tasks, m.cursor)
	// } else {
	// 	s += RenderTaskList(m.context.Tasks, m.cursor)
	// }

	app := lipgloss.JoinVertical(
		lipgloss.Top,
		header,
		content.Render(s),
		addWidget,
	)

	return app
}
