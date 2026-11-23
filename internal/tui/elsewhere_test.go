package tui

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bricef/htt/internal/domain"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
)

// While the TUI is open, another htt (the CLI, the REPL, a second TUI)
// can change the file. Every action that changes tasks works on the
// file as it is when the key is pressed, on the task the TUI showed:
// nothing written elsewhere is lost, and a task changed or removed
// elsewhere is left alone.

// fileTreeModel is seedTreeModel backed by files, which another htt
// can change underneath (elsewhere).
func fileTreeModel(t *testing.T, tasks ...string) model {
	t.Helper()
	m, _ := seedFileModel(t, "todo", tasks...)
	m.now = func() time.Time { return listNow }
	m.list.FilterInput.Cursor.SetMode(cursor.CursorStatic)
	m.textInput.Cursor.SetMode(cursor.CursorStatic)
	m.showTasks(-1)
	m.width, m.height = 100, 30
	m.layout()
	return m
}

// elsewhere changes the todo file as another htt would: it loads it
// afresh, and change saves it.
func elsewhere(t *testing.T, m model, change func(ctx *domain.Context) error) {
	t.Helper()
	ctx, err := m.repo.Context("todo")
	if err != nil {
		t.Fatal(err)
	}
	if err := change(ctx); err != nil {
		t.Fatalf("the change elsewhere: %v", err)
	}
}

func addElsewhere(text string) func(ctx *domain.Context) error {
	return func(ctx *domain.Context) error {
		task, err := domain.NewTask(text)
		if err != nil {
			return err
		}
		return ctx.AddTask(task)
	}
}

// lines is the todo file's lines, as loaded.
func lines(t *testing.T, m model, name string) []string {
	t.Helper()
	ctx, err := m.repo.Context(name)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, task := range ctx.Tasks {
		out = append(out, task.Raw)
	}
	return out
}

// keys sends each message through Update, settling the commands it
// starts, as a terminal would.
func keys(t *testing.T, m model, msgs ...tea.Msg) model {
	t.Helper()
	for _, msg := range msgs {
		m = settle(t, m, msg)
	}
	return m
}

func runes(s string) []tea.Msg {
	var out []tea.Msg
	for _, r := range s {
		out = append(out, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return out
}

// The actions that change tasks, each on a task of launch (see
// subtasks_test.go): the task's index, the keys, and the lines of the
// todo file the action leaves, the line added elsewhere aside.
var changingActions = []struct {
	name  string
	index int
	keys  []tea.Msg
	want  []string
}{
	{"x", 4, runes("x"), launch[:4]},
	{"x, then x", 0, runes("xx"), []string{"Write the announcement p:1 +work", "Book the venue p:1 id:2", "Call three venues p:2", "Pack the van"}},
	{"x, then A", 0, runes("xA"), []string{"Pack the van"}},
	{"d", 2, runes("d"), []string{launch[0], launch[1], launch[4]}},
	{"+", 4, runes("+"), []string{launch[0], launch[1], launch[2], launch[3], "(C) Pack the van"}},
	{"-", 0, runes("-"), []string{"(B) Plan the launch +work id:1", launch[1], launch[2], launch[3], launch[4]}},
	{"tab", 4, []tea.Msg{tab}, []string{launch[0], launch[1], launch[2], launch[3], "Pack the van p:1"}},
	{"shift+tab", 3, []tea.Msg{shiftTab}, []string{launch[0], launch[1], launch[2], "Call three venues p:1", launch[4]}},
	{"e", 1, append(append(runes("e"), runes(" now")...), enter), []string{launch[0], "Write the announcement p:1 +work now", launch[2], launch[3], launch[4]}},
	{"s", 4, append(append(runes("s"), runes("Load")...), enter), []string{launch[0], launch[1], launch[2], launch[3], "Pack the van id:3", "2026-05-16 Load p:3"}},
}

// A task added elsewhere while the TUI is open stays, whatever the TUI
// does next.
func TestElsewhere_ActionsKeepWhatElsewhereWrote(t *testing.T) {
	for _, a := range changingActions {
		t.Run(a.name, func(t *testing.T) {
			m := fileTreeModel(t, launch...)
			m = selectRow(t, m, a.index)
			elsewhere(t, m, addElsewhere("Added elsewhere"))
			m = keys(t, m, a.keys...)

			got := lines(t, m, "todo")
			added := slices.IndexFunc(got, func(l string) bool { return strings.HasSuffix(l, " Added elsewhere") })
			if added < 0 {
				t.Fatalf("the task added elsewhere is gone: %q (status %q)", got, m.status)
			}
			got = slices.Delete(got, added, added+1)
			if !slices.Equal(got, a.want) {
				t.Errorf("todo, the added task aside:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(a.want, "\n  "))
			}
		})
	}
}

// A task moved elsewhere (here, the file edited to put a task above
// them all) is acted on where it is now, by its line.
func TestElsewhere_ActionsFollowTheTaskShown(t *testing.T) {
	for _, a := range changingActions {
		t.Run(a.name, func(t *testing.T) {
			m := fileTreeModel(t, launch...)
			m = selectRow(t, m, a.index)
			edited := "(A) Zed\n" + strings.Join(launch, "\n") + "\n"
			if err := os.WriteFile(m.repo.ContextPath("todo"), []byte(edited), 0o644); err != nil {
				t.Fatal(err)
			}
			m = keys(t, m, a.keys...)
			want := append([]string{"(A) Zed"}, a.want...)
			if got := lines(t, m, "todo"); !slices.Equal(got, want) {
				t.Errorf("todo:\n  %s\nwant:\n  %s (status %q)", strings.Join(got, "\n  "), strings.Join(want, "\n  "), m.status)
			}
		})
	}
}

// A task changed or removed elsewhere is left alone: nothing changes,
// the list shows the file as it is, and the status line says why.
func TestElsewhere_ATaskChangedElsewhereIsLeftAlone(t *testing.T) {
	changes := map[string]func(ctx *domain.Context, index int) error{
		"changed": func(ctx *domain.Context, index int) error {
			task, err := domain.NewTask(ctx.Tasks[index].Raw + " +changed")
			if err != nil {
				return err
			}
			_, err = ctx.Replace(strID(index), task)
			return err
		},
		"removed": func(ctx *domain.Context, index int) error {
			_, err := ctx.Delete(strID(index))
			return err
		},
	}
	for _, a := range changingActions {
		for how, change := range changes {
			t.Run(a.name+", "+how, func(t *testing.T) {
				m := fileTreeModel(t, launch...)
				m = selectRow(t, m, a.index)
				// The change lands before the key that acts: while the
				// question is up, or the task input open.
				last := len(a.keys) - 1
				m = keys(t, m, a.keys[:last]...)
				elsewhere(t, m, func(ctx *domain.Context) error { return change(ctx, a.index) })
				before := lines(t, m, "todo")
				next, cmd := m.Update(a.keys[last])
				m = asModel(t, next)
				if isQuit(cmd) {
					t.Fatal("the TUI quit")
				}
				if got := lines(t, m, "todo"); !slices.Equal(got, before) {
					t.Errorf("todo changed:\n  %s\nwas:\n  %s", strings.Join(got, "\n  "), strings.Join(before, "\n  "))
				}
				if !strings.Contains(m.status, "changed") {
					t.Errorf("status %q doesn't say the task changed", m.status)
				}
				if m.input != inputClosed {
					t.Error("the task input is still open")
				}
				var shown []string
				for _, item := range m.list.Items() {
					shown = append(shown, item.(taskItem).task.Raw)
				}
				if !slices.Equal(shown, before) {
					t.Errorf("the list shows\n  %s\nnot the file:\n  %s", strings.Join(shown, "\n  "), strings.Join(before, "\n  "))
				}
			})
		}
	}
}

// The review's case: elsewhere, a task is added and raised to (A),
// which sorts it to the top; then s on Book, in the TUI's list as it
// was, adds the subtask under Book, not under the task now at Book's
// old number.
func TestElsewhere_ASubtaskGoesUnderTheTaskShown(t *testing.T) {
	m := fileTreeModel(t, "Plan id:1", "Write p:1", "Book p:1 id:2", "Call p:2", "Other", "Last")
	elsewhere(t, m, addElsewhere("Zed"))
	elsewhere(t, m, func(ctx *domain.Context) error {
		_, _, err := ctx.SetPriority("6", "A")
		return err
	})
	m = selectRow(t, m, 2) // Book, as the TUI shows it
	m = keys(t, m, runes("s")...)
	m = keys(t, m, runes("child")...)
	m = keys(t, m, enter)

	got := lines(t, m, "todo")
	if len(got) != 8 || !strings.HasSuffix(got[0], " Zed") {
		t.Fatalf("todo: %q", got)
	}
	want := []string{"Plan id:1", "Write p:1", "Book p:1 id:2", "Call p:2", "2026-05-16 child p:2", "Other", "Last"}
	if !slices.Equal(got[1:], want) {
		t.Errorf("todo:\n  %s\nwant, after (A) Zed:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// Tab nests a task under the task the TUI showed above it. When another
// task is above it in the file now, nothing changes.
func TestElsewhere_TabNestsOnlyUnderTheTaskShownAbove(t *testing.T) {
	m := fileTreeModel(t, "Plan", "Other", "Pack")
	m = selectRow(t, m, 2) // Pack, below Other
	elsewhere(t, m, func(ctx *domain.Context) error {
		_, _, err := ctx.SetPriority("1", "A") // (A) Other sorts first: Plan is above Pack now
		return err
	})
	before := lines(t, m, "todo")
	if !slices.Equal(before, []string{"(A) Other", "Plan", "Pack"}) {
		t.Fatalf("todo: %q", before)
	}
	m = keys(t, m, tab)
	if got := lines(t, m, "todo"); !slices.Equal(got, before) {
		t.Errorf("todo changed: %q, was %q", got, before)
	}
	if !strings.Contains(m.status, "changed") {
		t.Errorf("status %q", m.status)
	}
}

// The question about open subtasks is about the ones it counted: when
// they change before the answer, nothing is completed.
func TestElsewhere_TheAnswerIsAboutTheSubtasksAskedAbout(t *testing.T) {
	m := fileTreeModel(t, launch...)
	m = keys(t, m, runes("x")...)
	viewHas(t, m, "Task 0 has 3 open subtasks.")
	elsewhere(t, m, func(ctx *domain.Context) error { return ctx.AddSubtask("0", mustTask(t, "Hire a van")) })
	before := lines(t, m, "todo")
	m = keys(t, m, runes("A")...)
	if got := lines(t, m, "todo"); !slices.Equal(got, before) {
		t.Errorf("todo changed: %q", got)
	}
	if got := lines(t, m, domain.DoneContextName); len(got) != 0 {
		t.Errorf("done: %q", got)
	}
	viewHas(t, m, "changed")
}

// The answer completes the task asked about, wherever it is now.
func TestElsewhere_TheAnswerFollowsTheTaskAskedAbout(t *testing.T) {
	m := fileTreeModel(t, launch...)
	m = keys(t, m, runes("x")...)
	viewHas(t, m, "Task 0 has 3 open subtasks.")
	edited := "(A) Zed\n" + strings.Join(launch, "\n") + "\n"
	if err := os.WriteFile(m.repo.ContextPath("todo"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	m = keys(t, m, runes("A")...)
	if got := lines(t, m, "todo"); !slices.Equal(got, []string{"(A) Zed", "Pack the van"}) {
		t.Errorf("todo: %q", got)
	}
	viewHas(t, m, "Completed task 0 and 3 open subtasks.")
}
