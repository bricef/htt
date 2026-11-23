package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bricef/htt/internal/domain"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The design's example, with a task after it.
var launch = []string{
	"(A) Plan the launch +work id:1",
	"Write the announcement p:1 +work",
	"Book the venue p:1 id:2",
	"Call three venues p:2",
	"Pack the van",
}

func seedTreeModel(t *testing.T, tasks ...string) (model, *storageRepo) {
	t.Helper()
	m, repo := seedModel(t, "todo", tasks...)
	m.now = func() time.Time { return listNow }
	m.list.FilterInput.Cursor.SetMode(cursor.CursorStatic)
	m.showTasks(-1)
	m.width, m.height = 100, 30
	m.layout()
	return m, &storageRepo{repo}
}

// rows is the list as drawn, without colours, each row trimmed on the
// right.
func rows(m model) []string {
	var out []string
	for _, l := range strings.Split(stripANSIForTest(m.list.View()), "\n") {
		if l = strings.TrimRight(l, " "); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func wantRows(t *testing.T, label string, m model, want ...string) {
	t.Helper()
	if got := rows(m); !slices.Equal(got, want) {
		t.Errorf("%s:\n  %s\nwant:\n  %s", label, strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func hit(t *testing.T, m model, k tea.KeyMsg) model {
	t.Helper()
	return settle(t, m, k)
}

var (
	shiftTab = tea.KeyMsg{Type: tea.KeyShiftTab}
	space    = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	down     = tea.KeyMsg{Type: tea.KeyDown}
)

// selectRow moves the cursor to the task with index i.
// viewHas checks the view, without colours, shows want.
func viewHas(t *testing.T, m model, want string) {
	t.Helper()
	if view := stripANSIForTest(m.View()); !strings.Contains(view, want) {
		t.Errorf("view should contain %q, got:\n%s", want, view)
	}
}

func selectRow(t *testing.T, m model, i int) model {
	t.Helper()
	m.selectTask(i)
	if it, ok := m.selectedTask(); !ok || it.index != i {
		t.Fatalf("task %d isn't on show", i)
	}
	return m
}

func selected(t *testing.T, m model) string {
	t.Helper()
	it, ok := m.selectedTask()
	if !ok {
		t.Fatal("nothing selected")
	}
	return it.task.Raw
}

func TestTree_ShownUnfolded(t *testing.T) {
	m, _ := seedTreeModel(t, launch...)
	wantRows(t, "the tree", m,
		">    0 (A) Plan the launch +work id:1",
		"     1   Write the announcement p:1 +work",
		"     2   Book the venue p:1 id:2",
		"     3     Call three venues p:2",
		"     4 Pack the van",
	)
}

func TestTree_FoldAndUnfold(t *testing.T) {
	m, _ := seedTreeModel(t, launch...)
	m = hit(t, m, space) // Plan
	wantRows(t, "Plan folded", m,
		">    0 (A) Plan the launch +work id:1  (3 folded)",
		"     4 Pack the van",
	)
	// The next row down is the next task on show.
	m = hit(t, m, down)
	if got := selected(t, m); got != "Pack the van" {
		t.Errorf("down from a folded task: %q", got)
	}
	m = hit(t, m, tea.KeyMsg{Type: tea.KeyUp})
	m = hit(t, m, space)
	wantRows(t, "unfolded", m,
		">    0 (A) Plan the launch +work id:1",
		"     1   Write the announcement p:1 +work",
		"     2   Book the venue p:1 id:2",
		"     3     Call three venues p:2",
		"     4 Pack the van",
	)

	m = selectRow(t, m, 2)
	m = hit(t, m, space) // Book
	wantRows(t, "Book folded", m,
		"     0 (A) Plan the launch +work id:1",
		"     1   Write the announcement p:1 +work",
		">    2   Book the venue p:1 id:2  (1 folded)",
		"     4 Pack the van",
	)

	m = selectRow(t, m, 4)
	m = hit(t, m, space) // Pack: nothing to fold
	viewHas(t, m, "Task 4 has no subtasks to fold.")
}

// A search looks through folded tasks too, its rows flat; when it ends,
// the list folds again.
func TestTree_SearchLooksInsideFolds(t *testing.T) {
	m, _ := seedTreeModel(t, launch...)
	m = hit(t, m, space) // fold Plan
	m = search(t, m, "venue")
	wantRows(t, "searching", m,
		"search: venue · 2 of 5 (esc clears)",
		">    2 Book the venue p:1 id:2",
		"     3 Call three venues p:2",
	)
	m = hit(t, m, down)
	m = hit(t, m, keyPress("esc"))
	// Call was selected: what's above it unfolds, so it stays on show.
	wantRows(t, "after the search", m,
		"     0 (A) Plan the launch +work id:1",
		"     1   Write the announcement p:1 +work",
		"     2   Book the venue p:1 id:2",
		">    3     Call three venues p:2",
		"     4 Pack the van",
	)
}

func TestTree_NewSubtask(t *testing.T) {
	m, repo := seedTreeModel(t, launch...)
	m = selectRow(t, m, 4) // Pack the van: no id yet
	m = press(t, m, "s")
	viewHas(t, m, "New subtask of 4 > ")
	m = press(t, m, "Load the boxes")
	viewHas(t, m, "saves as 2026-05-16 Load the boxes p:3")
	m = hit(t, m, keyPress("enter"))
	wantRows(t, "added", m,
		"     0 (A) Plan the launch +work id:1",
		"     1   Write the announcement p:1 +work",
		"     2   Book the venue p:1 id:2",
		"     3     Call three venues p:2",
		"     4 Pack the van id:3",
		">    5   2026-05-16 Load the boxes p:3",
	)
	_ = repo

	// Under a folded task: it unfolds, to show the new one.
	m = selectRow(t, m, 2)
	m = hit(t, m, space)
	m = press(t, m, "s")
	m = press(t, m, "(A) Ring Ann")
	m = hit(t, m, keyPress("enter"))
	if got := selected(t, m); got != "(A) 2026-05-16 Ring Ann p:2" {
		t.Errorf("selected %q after adding under a folded task", got)
	}
	wantRows(t, "added at its priority's place", m,
		"     0 (A) Plan the launch +work id:1",
		"     1   Write the announcement p:1 +work",
		"     2   Book the venue p:1 id:2",
		">    3     (A) 2026-05-16 Ring Ann p:2",
		"     4     Call three venues p:2",
		"     5 Pack the van id:3",
		"     6   2026-05-16 Load the boxes p:3",
	)

	// Not in the done tab.
	m.context = &domain.Context{Name: domain.DoneContextName}
	m = press(t, m, "s")
	if m.input != inputClosed || !strings.Contains(m.status, "The done tab doesn't hold subtasks.") {
		t.Errorf("s in the done tab: input %v, status %q", m.input, m.status)
	}
}

func TestTree_TabNestsUnderTheTaskAbove(t *testing.T) {
	m, repo := seedTreeModel(t, launch...)

	// Pack the van, a top-level task: under the top-level task above it,
	// past Plan's subtasks.
	m = selectRow(t, m, 4)
	m = hit(t, m, tab)
	if got := selected(t, m); got != "Pack the van p:1" {
		t.Errorf("selected %q after Tab", got)
	}
	wantRows(t, "Pack under Plan", m,
		"     0 (A) Plan the launch +work id:1",
		"     1   Write the announcement p:1 +work",
		"     2   Book the venue p:1 id:2",
		"     3     Call three venues p:2",
		">    4   Pack the van p:1",
	)

	// Book, with its subtree, under Write, which gets an id.
	m = selectRow(t, m, 2)
	m = hit(t, m, tab)
	wantRows(t, "Book under Write", m,
		"     0 (A) Plan the launch +work id:1",
		"     1   Write the announcement p:1 +work id:3",
		">    2     Book the venue p:3 id:2",
		"     3       Call three venues p:2",
		"     4   Pack the van p:1",
	)

	// The first at its level: nothing changes, and the status says why.
	before := repo.raws(t, "todo")
	for _, i := range []int{0, 1, 3} {
		m = selectRow(t, m, i)
		m = hit(t, m, tab)
		viewHas(t, m, "nothing above it to nest it under.")
	}
	if got := repo.raws(t, "todo"); !slices.Equal(got, before) {
		t.Errorf("Tab on a first task changed the file:\n%q", got)
	}

	// Under a folded task: it unfolds, so the task stays on show.
	m = selectRow(t, m, 1)
	m = hit(t, m, space) // fold Write
	m = selectRow(t, m, 4)
	m = hit(t, m, tab)
	if got := selected(t, m); got != "Pack the van p:3" {
		t.Errorf("selected %q after Tab under a folded task", got)
	}
}

func TestTree_ShiftTabUnnestsALevel(t *testing.T) {
	m, _ := seedTreeModel(t, append(launch, "Ring Ann p:2")...)
	// 0 Plan, 1 Write, 2 Book, 3 Call, 4 Ring Ann, 5 Pack.

	m = selectRow(t, m, 3) // Call: up to Plan, after Book's subtree
	m = hit(t, m, shiftTab)
	wantRows(t, "Call up a level", m,
		"     0 (A) Plan the launch +work id:1",
		"     1   Write the announcement p:1 +work",
		"     2   Book the venue p:1 id:2",
		"     3     Ring Ann p:2",
		">    4   Call three venues p:1",
		"     5 Pack the van",
	)

	m = selectRow(t, m, 2) // Book, with Ring Ann: top level, after Plan's subtree
	m = hit(t, m, shiftTab)
	wantRows(t, "Book to the top level", m,
		"     0 (A) Plan the launch +work id:1",
		"     1   Write the announcement p:1 +work",
		"     2   Call three venues p:1",
		">    3 Book the venue id:2",
		"     4   Ring Ann p:2",
		"     5 Pack the van",
	)

	m = hit(t, m, shiftTab)
	viewHas(t, m, "Task 3 is already a top-level task.")
	if got := selected(t, m); got != "Book the venue id:2" {
		t.Errorf("selected %q", got)
	}
}

func TestTree_DoAsksAboutOpenSubtasks(t *testing.T) {
	question := []string{
		"Task 0 has 3 open subtasks.",
		"[x] Mark it done, and leave the subtasks as they are.",
		"[A] Mark the subtasks done too.",
		"Anything else cancels.",
	}

	t.Run("x: the task alone", func(t *testing.T) {
		m, repo := seedTreeModel(t, launch...)
		m = press(t, m, "x")
		for _, line := range question {
			viewHas(t, m, line)
		}
		viewHas(t, m, "other keys cancel")
		if got := repo.raws(t, "todo"); len(got) != 5 {
			t.Fatalf("asking changed the file: %q", got)
		}
		m = press(t, m, "x")
		viewHas(t, m, "Completed task 0; its subtasks stay open.")
		wantRows(t, "after x", m,
			">    0 Write the announcement p:1 +work  (parent done)",
			"     1 Book the venue p:1 id:2  (parent done)",
			"     2   Call three venues p:2",
			"     3 Pack the van",
		)
	})

	t.Run("a folded task under a done parent says both", func(t *testing.T) {
		m, _ := seedTreeModel(t, launch...)
		m = selectRow(t, m, 2)
		m = hit(t, m, space) // fold Book
		m = selectRow(t, m, 0)
		m = press(t, m, "xx")
		wantRows(t, "after x", m,
			">    0 Write the announcement p:1 +work  (parent done)",
			"     1 Book the venue p:1 id:2  (parent done)  (1 folded)",
			"     3 Pack the van",
		)
	})

	t.Run("A: every open task below", func(t *testing.T) {
		m, repo := seedTreeModel(t, launch...)
		m = press(t, m, "xA")
		viewHas(t, m, "Completed task 0 and 3 open subtasks.")
		if got := repo.raws(t, "todo"); !slices.Equal(got, []string{"Pack the van"}) {
			t.Errorf("todo: %q", got)
		}
		if got := repo.raws(t, domain.DoneContextName); len(got) != 4 {
			t.Errorf("done: %q", got)
		}
	})

	for _, k := range []tea.KeyMsg{keyPress("a"), keyPress("X"), keyPress("d"), keyPress("esc"), keyPress("enter"), space} {
		t.Run("cancelled by "+k.String(), func(t *testing.T) {
			m, repo := seedTreeModel(t, launch...)
			m = press(t, m, "x")
			m = hit(t, m, k)
			viewHas(t, m, "Cancelled: nothing changed.")
			if got := repo.raws(t, "todo"); len(got) != 5 || m.asking != nil {
				t.Errorf("todo: %q, still asking: %v", got, m.asking != nil)
			}
			if m.input != inputClosed {
				t.Error("the key that cancelled also opened the input")
			}
		})
	}

	t.Run("no open subtasks: done at once", func(t *testing.T) {
		m, repo := seedTreeModel(t, launch...)
		m = selectRow(t, m, 4)
		m = press(t, m, "x")
		if m.asking != nil || len(repo.raws(t, "todo")) != 4 {
			t.Errorf("asked, or didn't complete: %q", repo.raws(t, "todo"))
		}
	})
}

func TestTree_DeleteTakesTheSubtree(t *testing.T) {
	m, repo := seedTreeModel(t, launch...)
	m = selectRow(t, m, 2)
	m = press(t, m, "d")
	viewHas(t, m, "Deleted task 2 and 1 subtask.")
	if got := repo.raws(t, "todo"); len(got) != 3 {
		t.Errorf("todo: %q", got)
	}
}

// Raising a subtask's priority moves it among its siblings, and the
// cursor goes with it.
func TestTree_PriorityKeysFollowTheTask(t *testing.T) {
	m, _ := seedTreeModel(t, launch...)
	m = selectRow(t, m, 2) // Book
	m = press(t, m, "+")
	if got := selected(t, m); got != "(C) Book the venue p:1 id:2" {
		t.Errorf("selected %q after +", got)
	}
	wantRows(t, "Book raised", m,
		"     0 (A) Plan the launch +work id:1",
		">    1   (C) Book the venue p:1 id:2",
		"     2     Call three venues p:2",
		"     3   Write the announcement p:1 +work",
		"     4 Pack the van",
	)
	m = press(t, m, "-")
	if got := selected(t, m); got != "Book the venue p:1 id:2" {
		t.Errorf("selected %q after -", got)
	}
}

// Broken links are pointed out on the status line, as dates htt can't
// read are.
func TestTree_BrokenLinksOnTheStatusLine(t *testing.T) {
	m, _ := seedTreeModel(t, "fine", "lost p:9", "2026-13-45 bad date")
	if want := "Task 1: p:9 names no task, so the task is shown at the top level (and 1 more warning in this list)."; m.status != want {
		t.Errorf("status %q, want %q", m.status, want)
	}
}

// id: and p: are drawn muted; the rest of the line keeps its colours.
func TestTree_LinkTagsAreMuted(t *testing.T) {
	trueColor(t)
	theme := themes[DefaultThemeName]
	st := newStyles(theme)
	raw := "(A) Plan +work id:1 p:3 due:2026-05-20 @home"
	drawn := cells(st.line(raw, domain.Tokens(raw), nil, false))
	want := map[string]string{
		"(A)": string(theme.PriorityA), "Plan": "", "+work": string(theme.PlusTag),
		"id:1": string(theme.Muted), "p:3": string(theme.Muted),
		"due:2026-05-20": string(theme.Annotation), "@home": string(theme.AtTag),
	}
	at := 0
	for _, word := range strings.Fields(raw) {
		for i := range len([]rune(word)) {
			if c := drawn[at+i]; !sameColour(c.fg, lipgloss.Color(want[word])) {
				t.Errorf("%q drawn in %q, want %q", word, c.fg, want[word])
				break
			}
		}
		at += len([]rune(word)) + 1
	}
}
