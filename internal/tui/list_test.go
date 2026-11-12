package tui

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/storage"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
)

// storageRepo reads back what actions stored.
type storageRepo struct{ *storage.MemoryRepository }

// raws returns the stored lines of a context, in order.
func (r *storageRepo) raws(t *testing.T, name string) []string {
	t.Helper()
	ctx, err := r.Context(name)
	if err != nil {
		t.Fatalf("Context(%q): %v", name, err)
	}
	out := make([]string, len(ctx.Tasks))
	for i, task := range ctx.Tasks {
		out[i] = task.Raw
	}
	return out
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSIForTest(s string) string { return ansiRE.ReplaceAllString(s, "") }

// listNow anchors due-date filter words: 2026-05-16, mid-afternoon.
var listNow = time.Date(2026, 5, 16, 15, 0, 0, 0, time.Local)

// settle delivers msg, then runs the commands it produces and feeds
// their messages back, the way Bubble Tea's loop would, so work the list
// does asynchronously (search results) lands. Commands that don't return
// within 50ms (timers) are dropped.
func settle(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	queue := []tea.Msg{msg}
	for steps := 0; len(queue) > 0 && steps < 200; steps++ {
		next, cmd := m.Update(queue[0])
		queue = queue[1:]
		m = asModel(t, next)
		queue = append(queue, runCmd(cmd)...)
	}
	return m
}

func runCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case msg := <-ch:
		switch msg := msg.(type) {
		case nil, tea.QuitMsg:
			return nil
		case tea.BatchMsg:
			var out []tea.Msg
			for _, c := range msg {
				out = append(out, runCmd(c)...)
			}
			return out
		default:
			return []tea.Msg{msg}
		}
	case <-time.After(50 * time.Millisecond):
		return nil
	}
}

func keyPress(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// search types a search into the TUI as a user would: "/", the query,
// Enter.
func search(t *testing.T, m model, query string) model {
	t.Helper()
	m = settle(t, m, keyPress("/"))
	for _, r := range query {
		m = settle(t, m, keyPress(string(r)))
	}
	return settle(t, m, keyPress("enter"))
}

// shown lists the texts of the tasks the list shows, in order.
func shown(m model) []string {
	var out []string
	for _, item := range m.list.VisibleItems() {
		out = append(out, item.(taskItem).task.Entry())
	}
	return out
}

func seedSearchModel(t *testing.T, tasks ...string) (model, *storageRepo) {
	t.Helper()
	m, repo := seedModel(t, "todo", tasks...)
	m.now = func() time.Time { return listNow }
	// A steady cursor in the search box, so no blink timers to wait out.
	m.list.FilterInput.Cursor.SetMode(cursor.CursorStatic)
	m.showTasks(-1)
	m.width, m.height = 100, 30
	m.layout()
	return m, &storageRepo{repo}
}

func TestSearch_NarrowsByTextFilterWordsAndBoth(t *testing.T) {
	tasks := []string{
		"(A) buy milk due:2026-05-15",
		"(B) buy bread +shop",
		"call the plumber @home due:2026-05-20",
		"(A) water the plants @home",
		"Milk the goat",
	}
	cases := []struct {
		query string
		want  []string
	}{
		{"milk", []string{"buy milk", "Milk the goat"}},
		{"MILK", []string{"buy milk", "Milk the goat"}}, // case-insensitive
		{"buy milk", []string{"buy milk"}},              // every word must match
		{"priority:A", []string{"buy milk", "water the plants @home"}},
		{"has-tag:home", []string{"call the plumber @home", "water the plants @home"}},
		{"overdue", []string{"buy milk"}},
		{"due-soon", []string{"call the plumber @home"}},
		{"priority:A home", []string{"water the plants @home"}}, // filter word AND text
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			m, _ := seedSearchModel(t, tasks...)
			m = search(t, m, tc.query)
			if got := shown(m); !slices.Equal(got, tc.want) {
				t.Errorf("search %q shows %q, want %q", tc.query, got, tc.want)
			}
		})
	}
}

func TestSearch_NoMatchShowsNothingWhileTypingAndEnterClearsIt(t *testing.T) {
	// The list component's own behaviour: Enter on a search with no
	// results drops the search rather than leaving an empty list.
	m, _ := seedSearchModel(t, "alpha", "beta")
	m = settle(t, m, keyPress("/"))
	for _, r := range "zzz" {
		m = settle(t, m, keyPress(string(r)))
	}
	if got := shown(m); len(got) != 0 {
		t.Errorf("while typing a search with no match, shows %q, want nothing", got)
	}
	m = settle(t, m, keyPress("enter"))
	if m.list.IsFiltered() || len(shown(m)) != 2 {
		t.Errorf("enter on an empty search should clear it; filtered=%v shown=%q", m.list.IsFiltered(), shown(m))
	}
}

func TestSearch_ShowsTheAppliedSearchAndKeepsTaskNumbers(t *testing.T) {
	m, _ := seedSearchModel(t, "alpha", "beta milk", "gamma", "delta milk")
	m = search(t, m, "milk")

	view := stripANSIForTest(m.View())
	if !strings.Contains(view, "search: milk · 2 of 4 (esc clears)") {
		t.Errorf("view should show the applied search:\n%s", view)
	}
	// Rows keep their task numbers: 1 and 3, not 0 and 1.
	if !strings.Contains(view, "   1 beta milk") || !strings.Contains(view, "   3 delta milk") {
		t.Errorf("rows should show each task's number in the context:\n%s", view)
	}
}

func TestSearch_ActionsHitTheSelectedTaskNotItsRow(t *testing.T) {
	// The search shows tasks 1 and 3; the cursor is on the second row,
	// which is task 3. Acting on row 1 of the view must reach task 3.
	cases := []struct {
		name   string
		action *Action
		check  func(t *testing.T, todo, done []string)
	}{
		{"do", Do, func(t *testing.T, todo, done []string) {
			if !slices.Equal(todo, []string{"alpha", "beta milk", "gamma"}) || len(done) != 1 || !strings.Contains(done[0], "delta milk") {
				t.Errorf("todo=%q done=%q, want delta milk completed", todo, done)
			}
		}},
		{"delete", Delete, func(t *testing.T, todo, _ []string) {
			if !slices.Equal(todo, []string{"alpha", "beta milk", "gamma"}) {
				t.Errorf("todo=%q, want delta milk deleted", todo)
			}
		}},
		{"increase priority", IncreasePriority, func(t *testing.T, todo, _ []string) {
			if !slices.Contains(todo, "(C) delta milk") || slices.Contains(todo, "(C) beta milk") {
				t.Errorf("todo=%q, want delta milk raised to (C)", todo)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, repo := seedSearchModel(t, "alpha", "beta milk", "gamma", "delta milk")
			m = search(t, m, "milk")
			m = settle(t, m, keyPress("j"))
			if it, _ := m.selectedTask(); it.index != 3 {
				t.Fatalf("selected task %d, want 3", it.index)
			}

			next, _ := tc.action.Act(m)
			m = asModel(t, next)
			tc.check(t, repo.raws(t, "todo"), repo.raws(t, domain.DoneContextName))

			if m.list.FilterValue() != "milk" || !m.list.IsFiltered() {
				t.Errorf("the search should stay applied after %s", tc.name)
			}
		})
	}
}

func TestSearch_EscClearsTheSearchThenQuits(t *testing.T) {
	m, _ := seedSearchModel(t, "alpha", "beta milk")
	m = search(t, m, "milk")

	next, cmd := m.Update(keyPress("esc"))
	m = asModel(t, next)
	if isQuit(cmd) || m.list.IsFiltered() || len(shown(m)) != 2 {
		t.Fatalf("first esc should clear the search (quit=%v filtered=%v shown=%q)", isQuit(cmd), m.list.IsFiltered(), shown(m))
	}
	if _, cmd := m.Update(keyPress("esc")); !isQuit(cmd) {
		t.Errorf("esc with no search should quit, as before")
	}
}

func TestList_PagingUsesPageKeysAndLeavesHttKeysAlone(t *testing.T) {
	var tasks []string
	for i := 0; i < 40; i++ {
		tasks = append(tasks, "task "+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	m, _ := seedSearchModel(t, tasks...)
	m.height = 15 // a few rows per page
	m.layout()
	perPage := m.list.Paginator.PerPage
	if perPage < 2 || perPage >= 40 {
		t.Fatalf("PerPage = %d, want a small page for this test", perPage)
	}

	m = settle(t, m, keyPress("pgdown"))
	if m.list.Index() != perPage {
		t.Errorf("pgdown: index = %d, want %d (the next page)", m.list.Index(), perPage)
	}
	m = settle(t, m, keyPress("pgup"))
	if m.list.Index() != 0 {
		t.Errorf("pgup: index = %d, want 0", m.list.Index())
	}
	// l and h switch contexts (htt), not pages (the list's default).
	m = settle(t, m, keyPress("l"))
	if m.contextCursor == 0 {
		t.Errorf("l should move to the next context tab")
	}
}

func TestList_PriorityChangeKeepsTheCursorOnTheTask(t *testing.T) {
	// Raising "b" (last) to (C) sorts it up to second place; the cursor
	// follows it rather than staying on its old row.
	m, _ := seedSearchModel(t, "(A) a", "c", "d", "b")
	m.list.Select(3)

	next, _ := IncreasePriority.Act(m)
	m = asModel(t, next)
	it, ok := m.selectedTask()
	if !ok || it.task.Entry() != "b" || it.task.Priority != "C" {
		t.Errorf("selected %q (priority %q), want the raised task b", it.task.Entry(), it.task.Priority)
	}
	if it.index != 1 {
		t.Errorf("b is at index %d, want 1: the test needs the raise to move it", it.index)
	}
}

func TestList_DoneTabHasNoTaskNumbers(t *testing.T) {
	m, _ := seedModel(t, domain.DoneContextName, "x 2026-05-15 finished thing")
	m.width, m.height = 100, 20
	view := stripANSIForTest(m.View())
	if !strings.Contains(view, "> x 2026-05-15 finished thing") {
		t.Errorf("done rows should have no number column:\n%s", view)
	}
}
