package tui

import (
	"log/slog"
	"slices"
	"time"

	"github.com/bricef/htt/internal/domain"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	lipgloss "github.com/charmbracelet/lipgloss"
)

// Options configures a TUI model. The zero value discards diagnostics
// and uses the default theme.
type Options struct {
	Logger *slog.Logger // diagnostics; nil discards them
	Theme  Theme        // the zero Theme means the default theme

	// UpdateNotice, if set, blocks until the update check has finished
	// and returns a one-line notice, or "" when there's nothing to say.
	// The TUI calls it in the background and shows the notice on the
	// status line.
	UpdateNotice func() string
}

// updateNoticeMsg carries a newer-version notice to Update.
type updateNoticeMsg string

type model struct {
	repo          domain.Repository
	context       *domain.Context
	list          list.Model // the current context's tasks; owns the cursor and the search
	contexts      []*domain.Context
	contextCursor int
	width         int
	height        int
	keys          KeyBindingController
	showHelp      bool
	newTask       bool
	textInput     textinput.Model
	logger        *slog.Logger // diagnostics; never the terminal
	status        string       // one-line message for the user; cleared on the next key
	styles        styles       // colours from the theme
	updateNotice  func() string
	now           func() time.Time // the clock searches use for due-date filters
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
	// Moving, paging and searching belong to the list (see listOwns);
	// these entries only list them in the help.
	AddShortBinding(Noop, listKeys().Filter).
	AddBinding(Noop, listKeys().CursorUp).
	AddBinding(Noop, listKeys().CursorDown).
	AddBinding(Noop, listKeys().PrevPage).
	AddBinding(Noop, listKeys().NextPage).
	AddBinding(Do, key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "do"),
	)).
	AddBinding(Delete, key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
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

// Model constructs the TUI model rooted at ctx, backed by repo for all
// mutating actions and context lookups. Diagnostics go to logger; a
// nil logger discards them. The TUI owns the terminal, so it never
// logs to stdout or stderr.
func Model(repo domain.Repository, ctx *domain.Context, opts Options) model {
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	theme := opts.Theme
	if theme == (Theme{}) {
		theme = themes[DefaultThemeName]
	}
	st := newStyles(theme)

	// Names-only view of the contexts; the tab strip only renders Name.
	// Tasks for each tab are loaded on demand when the user switches to it.
	names, _ := domain.SwitchableContextNames(repo)
	contexts := make([]*domain.Context, 0, len(names)+1)
	for _, name := range names {
		contexts = append(contexts, &domain.Context{Name: name})
	}
	contexts = append(contexts, &domain.Context{Name: domain.DoneContextName})

	selected := slices.IndexFunc(contexts, func(c *domain.Context) bool {
		return c.Equals(ctx)
	})
	// bug_006: On a fresh install the user opens the TUI before any
	// data files exist. ctx is the default "todo" context, but the
	// `contexts` slice contains only the synthetic "done" tab, so
	// IndexFunc returns -1. Pressing 'h' would then index
	// m.contexts[-1] in PreviousContext and panic. Clamp to 0 so the
	// initial tab strip always has a valid selection.
	if selected < 0 {
		selected = 0
	}

	ti := textinput.New()
	ti.Placeholder = "(A) Do a thing for +project in @context"

	ti.Prompt = "New Task > "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(theme.Background).Background(theme.Foreground)
	ti.PlaceholderStyle = ti.TextStyle.Foreground(theme.Muted)
	ti.Width = 100

	m := model{
		repo:          repo,
		context:       ctx,
		list:          newTaskList(st),
		contexts:      contexts,
		contextCursor: selected,
		keys:          controller,
		newTask:       false,
		textInput:     ti,
		logger:        logger,
		styles:        st,
		updateNotice:  opts.UpdateNotice,
		now:           time.Now,
	}
	m.showTasks(-1)
	return m
}

// Init waits for the update check in the background, if there is one.
func (m model) Init() tea.Cmd {
	notice := m.updateNotice
	if notice == nil {
		return nil
	}
	return func() tea.Msg {
		if s := notice(); s != "" {
			return updateNoticeMsg(s)
		}
		return nil
	}
}

// Update handles a message, then sizes the list to the space left
// between the header and the widgets below it.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	if nm, ok := next.(model); ok {
		nm.layout()
		return nm, cmd
	}
	return next, cmd
}

func (m model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case editorFinishedMsg:
		return editorFinished(m, msg)

	case updateNoticeMsg:
		m.status = string(msg)
		m.logger.Info("update available", "notice", string(msg))
		return m, nil

	case tea.KeyMsg:
		// A status message lasts until the user does something else.
		m.status = ""
		switch {
		case m.textInput.Focused():
			switch msg.Type {
			case tea.KeyEnter:
				return AddTask.Act(m)
			case tea.KeyEsc:
				return CancelNewTask.Act(m)
			case tea.KeyCtrlC:
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		case m.list.SettingFilter():
			// Typing a search: every key goes to the search box.
			if msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}
			return m.updateList(msg)
		case listOwns(msg, m.list.KeyMap), msg.Type == tea.KeyEsc && m.list.IsFiltered():
			// Moving, paging, opening a search, and Esc clearing one.
			return m.updateList(msg)
		}
		return m.keys.GetAction(msg).Act(m)
	}

	// Everything else (search results, the search box's cursor blink,
	// ...) belongs to the list.
	return m.updateList(msg)
}

// updateList passes a message to the list and keeps the search title
// line current.
func (m model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.syncSearchTitle()
	return m, cmd
}

// layout gives the list the height the other parts leave. Update and
// View both call it, so the list pages by what is on screen.
func (m *model) layout() {
	used := 0
	for _, part := range m.chrome() {
		used += lipgloss.Height(part)
	}
	m.list.SetSize(m.width, max(1, m.height-used))
}

// chrome is everything on screen except the task list, in no
// particular order: the header, the footer, and the new-task input and
// status line when they're showing.
func (m model) chrome() []string {
	parts := []string{m.headerView(), m.footerView()}
	for _, w := range []string{m.addWidgetView(), m.statusView()} {
		if w != "" {
			parts = append(parts, w)
		}
	}
	return parts
}

func (m model) View() string {
	m.layout()
	parts := []string{
		m.headerView(),
		lipgloss.NewStyle().Width(m.width).Height(m.list.Height()).Render(m.list.View()),
	}
	for _, w := range []string{m.addWidgetView(), m.statusView()} {
		if w != "" {
			parts = append(parts, w)
		}
	}
	parts = append(parts, m.footerView())
	return lipgloss.JoinVertical(lipgloss.Top, parts...)
}

func (m model) headerView() string {
	theme := m.styles.theme
	baseStyle := lipgloss.NewStyle()

	menuitem := baseStyle.Padding(0, 2).Align(lipgloss.Center)

	done := baseStyle.
		Inherit(menuitem).
		Padding(0, 2).
		Background(theme.Foreground).
		Foreground(theme.Background)

	menustr := ""
	for i, context := range m.contexts {
		itemStyle := menuitem
		if context.Name == "done" {
			itemStyle = done
		}
		if i == m.contextCursor {
			itemStyle = m.styles.selected(itemStyle)
		}

		menustr += itemStyle.Render(context.Name)

	}

	title := baseStyle.
		Align(lipgloss.Left).
		Foreground(theme.Foreground).
		Width(m.width / 2).
		Render("htt")

	menu := baseStyle.
		Align(lipgloss.Right).
		Width(m.width / 2).
		Render(menustr)

	return baseStyle.
		Align(lipgloss.Right).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(theme.Muted).
		Width(m.width).
		Render(title + menu)
}

func (m model) footerView() string {
	helpMenu := help.New()
	helpMenu.ShowAll = m.showHelp
	helpMenu.Styles = help.Styles{
		ShortKey:       m.styles.helpKey,
		ShortDesc:      m.styles.helpDesc,
		ShortSeparator: m.styles.helpSep,
		Ellipsis:       m.styles.helpSep,
		FullKey:        m.styles.helpKey,
		FullDesc:       m.styles.helpDesc,
		FullSeparator:  m.styles.helpSep,
	}

	return lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(m.width).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(m.styles.theme.Muted).
		Render(helpMenu.View(m.keys))
}

func (m model) addWidgetView() string {
	if !m.newTask {
		return ""
	}
	return m.textInput.View()
}

func (m model) statusView() string {
	if m.status == "" {
		return ""
	}
	return m.styles.status.Width(m.width).Render(m.status)
}
