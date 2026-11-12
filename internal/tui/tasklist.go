package tui

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/bricef/htt/internal/domain"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

const CURSOR = ">"

// taskItem is one row of the task list. index is the task's position
// in the context, which is what actions pass to the domain: a search
// changes which row a task is on, never its index.
type taskItem struct {
	task  *domain.Task
	index int
}

// FilterValue is what a search matches text against: the whole line.
func (i taskItem) FilterValue() string { return i.task.Raw }

// taskDelegate draws a row as "> NNNN text": the cursor, the task's
// index in its context (left out in the done tab) and the task's line
// (see styles.line). A row wider than the list ends in "…".
type taskDelegate struct {
	styles    styles
	showIndex bool
}

func (d taskDelegate) Height() int                         { return 1 }
func (d taskDelegate) Spacing() int                        { return 0 }
func (d taskDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d taskDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(taskItem)
	if !ok {
		return
	}
	selected := index == m.Index()
	cursor := " "
	if selected {
		cursor = CURSOR
	}
	text := d.styles.line(it.task.Raw, it.task.Tokens(), m.MatchesForItem(index), selected)
	row := fmt.Sprintf("%s %s", cursor, text)
	if d.showIndex {
		row = fmt.Sprintf("%s %4d %s", cursor, it.index, text)
	}
	if m.Width() > 0 {
		row = ansi.Truncate(row, m.Width(), "…")
	}
	_, _ = io.WriteString(w, row)
}

// line draws a task's line: each word in its kind's colour (see
// styles.token), or the whole line in the selected row's colours, with
// the characters a search matched underlined. tokens and matched are
// byte offsets into raw. Whitespace is drawn as plain spaces, so a tab
// or a stray carriage return can't break the row.
func (s styles) line(raw string, tokens []domain.Token, matched []int, selected bool) string {
	owner := make([]int, len(raw)) // the token each byte belongs to, or -1
	for i := range owner {
		owner[i] = -1
	}
	for ti, tk := range tokens {
		for i := tk.Start; i < tk.End && i < len(raw); i++ {
			owner[i] = ti
		}
	}
	hit := make([]bool, len(raw))
	for _, i := range matched {
		if i >= 0 && i < len(raw) {
			hit[i] = true
		}
	}

	var b strings.Builder
	draw := func(from, to int) {
		if from == to {
			return
		}
		st := s.plain
		switch {
		case selected:
			st = s.task
		case owner[from] >= 0:
			tk := tokens[owner[from]]
			st = s.token(tk.Kind, raw[tk.Start:tk.End])
		}
		if hit[from] {
			st = st.Underline(true)
		}
		text := raw[from:to]
		if owner[from] < 0 {
			text = strings.Map(func(rune) rune { return ' ' }, text)
		}
		b.WriteString(st.Render(text))
	}
	// Draw runs of characters that share a token and a match state.
	start := 0
	for i := range raw { // i steps over rune starts
		if owner[i] != owner[start] || hit[i] != hit[start] {
			draw(start, i)
			start = i
		}
	}
	draw(start, len(raw))
	return b.String()
}

// listKeys is the list's key map with htt's keys taken back: paging
// moves to PgUp/PgDn (the list's defaults include h, l, f and d), and
// quitting and help stay with htt.
func listKeys() list.KeyMap {
	km := list.DefaultKeyMap()
	km.PrevPage = key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "previous page"))
	km.NextPage = key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "next page"))
	km.Filter = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search"))
	km.ClearFilter = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear search"))
	disabled := key.NewBinding(key.WithDisabled())
	km.ShowFullHelp, km.CloseFullHelp, km.Quit, km.ForceQuit = disabled, disabled, disabled, disabled
	return km
}

// listOwns reports whether a key belongs to the list (moving, paging,
// opening the search) rather than to htt's own actions.
func listOwns(msg tea.KeyMsg, km list.KeyMap) bool {
	return key.Matches(msg, km.CursorUp, km.CursorDown, km.PrevPage, km.NextPage,
		km.GoToStart, km.GoToEnd, km.Filter)
}

func newTaskList(st styles) list.Model {
	l := list.New(nil, taskDelegate{styles: st, showIndex: true}, 0, 0)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetStatusBarItemName("task", "tasks")
	l.DisableQuitKeybindings()
	l.KeyMap = listKeys()
	l.FilterInput.Prompt = "search: "
	// The title line holds the search box while typing and the applied
	// search afterwards; see model.syncSearchTitle.
	l.Styles.TitleBar = lipgloss.NewStyle()
	l.Styles.Title = lipgloss.NewStyle().Foreground(st.theme.Muted)
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(st.theme.Accent).Bold(true)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(st.theme.Accent)
	l.Styles.NoItems = lipgloss.NewStyle().Foreground(st.theme.Muted).PaddingLeft(2)
	return l
}

// searchFilter is the list's filter function for a search over tasks.
// Each word of the search that is a filter expression (overdue,
// priority:A, has-tag:home, …; see domain.ParseFilter) must hold for
// the task, and each remaining word must fuzzily match its line. Tasks
// keep their file order, so the indexes on screen still ascend.
//
// targets are the items' filter values in item order, which is the
// order of tasks.
func searchFilter(tasks []*domain.Task, now func() time.Time) list.FilterFunc {
	return func(term string, targets []string) []list.Rank {
		var filters []domain.Filter
		var words []string
		for _, w := range strings.Fields(term) {
			if f, err := domain.ParseFilter(w, now()); err == nil {
				filters = append(filters, f)
			} else {
				words = append(words, w)
			}
		}

		ranks := []list.Rank{}
	tasks:
		for i, target := range targets {
			if i >= len(tasks) {
				break
			}
			for _, f := range filters {
				if !f(tasks[i]) {
					continue tasks
				}
			}
			var matched []int
			for _, w := range words {
				m := fuzzy.Find(w, []string{target})
				if len(m) == 0 {
					continue tasks
				}
				matched = append(matched, m[0].MatchedIndexes...)
			}
			slices.Sort(matched)
			ranks = append(ranks, list.Rank{Index: i, MatchedIndexes: slices.Compact(matched)})
		}
		return ranks
	}
}

// showTasks loads m.context's tasks into the list, keeping any applied
// search and, when select is a task index, putting the cursor on that
// task. The search is re-applied synchronously, so the list is ready to
// act on as soon as this returns.
func (m *model) showTasks(selectIndex int) {
	items := make([]list.Item, len(m.context.Tasks))
	for i, t := range m.context.Tasks {
		items[i] = taskItem{task: t, index: i}
	}
	m.list.SetDelegate(taskDelegate{
		styles:    m.styles,
		showIndex: m.context.Name != domain.DoneContextName,
	})
	m.list.Filter = searchFilter(m.context.Tasks, m.now)
	_ = m.list.SetItems(items)
	if m.list.IsFiltered() {
		position := m.list.Index()
		m.list.SetFilterText(m.list.FilterValue())
		m.list.Select(position)
	}
	if selectIndex >= 0 {
		m.selectTask(selectIndex)
	}
	m.clampSelection()
	m.syncSearchTitle()
}

// selectTask puts the cursor on the task with the given index in the
// context, if the list currently shows it.
func (m *model) selectTask(index int) {
	for pos, item := range m.list.VisibleItems() {
		if it, ok := item.(taskItem); ok && it.index == index {
			m.list.Select(pos)
			return
		}
	}
}

// clampSelection keeps the cursor on a row that exists after the list
// shrinks (bug_013: deleting or completing the last task used to leave
// the cursor past the end).
func (m *model) clampSelection() {
	if n := len(m.list.VisibleItems()); n > 0 && m.list.Index() >= n {
		m.list.Select(n - 1)
	}
}

// selectedTask returns the task under the cursor, with its index in the
// context; ok is false when the list (or the search) shows no task.
func (m model) selectedTask() (taskItem, bool) {
	it, ok := m.list.SelectedItem().(taskItem)
	return it, ok
}

// syncSearchTitle shows an applied search on the list's title line.
func (m *model) syncSearchTitle() {
	if m.list.FilterState() != list.FilterApplied {
		m.list.Title = ""
		return
	}
	m.list.Title = fmt.Sprintf("search: %s · %d of %d (esc clears)",
		m.list.FilterValue(), len(m.list.VisibleItems()), len(m.list.Items()))
}
