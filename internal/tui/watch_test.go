package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/storage"
	tea "github.com/charmbracelet/bubbletea"
)

// Watching the file shown (watch.go), and actions as transactions
// (onShown) racing other writers.

// shownLines are the lines of the tasks the list shows, in order.
func shownLines(m model) []string {
	var out []string
	for _, item := range m.list.Items() {
		out = append(out, item.(taskItem).task.Raw)
	}
	return out
}

// look delivers a tick of the watch and returns the model, failing the
// test if the watching stops.
func look(t *testing.T, m model) model {
	t.Helper()
	next, cmd := m.Update(watchMsg{})
	if cmd == nil {
		t.Fatal("the watch stopped: no next look")
	}
	return asModel(t, next)
}

// A change to the file elsewhere shows on the next look, the selection
// on the same task, though it moved.
func TestWatch_AChangeElsewhereShowsKeepingTheSelection(t *testing.T) {
	m := fileTreeModel(t, launch...)
	m = selectRow(t, m, 4) // Pack the van
	shown := m.context
	if m = look(t, m); m.context != shown {
		t.Error("the list was reloaded with nothing changed")
	}

	edited := "(A) Zed\n" + strings.Join(launch, "\n") + "\n"
	if err := os.WriteFile(m.repo.ContextPath("todo"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	m = look(t, m)
	if got := shownLines(m); !slices.Equal(got, append([]string{"(A) Zed"}, launch...)) {
		t.Errorf("the list shows\n  %s", strings.Join(got, "\n  "))
	}
	if it, ok := m.selectedTask(); !ok || it.task.Raw != "Pack the van" {
		t.Errorf("selected %q, want the task selected before, Pack the van", it.task.Raw)
	}
	if strings.Contains(m.View(), "Zed") == false {
		t.Errorf("the view doesn't show the new task:\n%s", m.View())
	}
}

// htt's own write elsewhere (a new file renamed over the old) shows
// too, and a selected task gone elsewhere leaves the cursor where it
// was.
func TestWatch_ASelectedTaskGoneLeavesTheCursorWhereItWas(t *testing.T) {
	m := fileTreeModel(t, "one", "two", "three")
	m = selectRow(t, m, 1)
	elsewhere(t, m, func(ctx *domain.Context) error {
		_, err := ctx.Delete("1")
		return err
	})
	m = look(t, m)
	if got := shownLines(m); !slices.Equal(got, []string{"one", "three"}) {
		t.Errorf("the list shows %q", got)
	}
	if got := m.list.Index(); got != 1 {
		t.Errorf("the cursor is on row %d, want 1, where it was", got)
	}
}

// An edit under way survives a reload: Enter saves it over the task it
// was opened on, though a reload moved it.
func TestWatch_AnEditUnderWaySurvivesAReload(t *testing.T) {
	m := fileTreeModel(t, launch...)
	m = selectRow(t, m, 1) // Write the announcement
	m = keys(t, m, runes("e")...)
	edited := "(A) Zed\n" + strings.Join(launch, "\n") + "\n"
	if err := os.WriteFile(m.repo.ContextPath("todo"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	m = look(t, m)
	m = keys(t, m, append(runes(" now"), enter)...)
	want := []string{"(A) Zed", launch[0], launch[1] + " now", launch[2], launch[3], launch[4]}
	if got := lines(t, m, "todo"); !slices.Equal(got, want) {
		t.Errorf("todo:\n  %s\nwant:\n  %s (status %q)", strings.Join(got, "\n  "), strings.Join(want, "\n  "), m.status)
	}
}

// A subtask input open across a reload stays on its task: the preview,
// the check before saving and the save are about the task it was opened
// for, wherever the reload moved it. Its index used to stay as it was:
// the preview named whatever task had that index now, and the check
// refused the save for that task's sake, or for an index past the end.
func TestWatch_ASubtaskInputFollowsItsTaskThroughAReload(t *testing.T) {
	for _, tc := range []struct {
		name    string
		tasks   []string
		row     int
		change  func(t *testing.T, m model)
		prompt  string
		preview string
		want    string
	}{
		{"a task above removed", append(slices.Clone(launch), "Zeta"), 5, func(t *testing.T, m model) {
			elsewhere(t, m, func(ctx *domain.Context) error {
				_, err := ctx.Delete("4") // Pack the van
				return err
			})
		}, "New subtask of 4 > ", "2026-05-16 sub p:3", "Zeta id:3"},
		{"a task put above, its old number naming a task with a shared id", []string{"Alpha id:7", "Beta id:7", "Gamma"}, 2, func(t *testing.T, m model) {
			edited := "(A) Zed\nAlpha id:7\nBeta id:7\nGamma\n"
			if err := os.WriteFile(m.repo.ContextPath("todo"), []byte(edited), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "New subtask of 3 > ", "2026-05-16 sub p:1", "Gamma id:1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fileTreeModel(t, tc.tasks...)
			m = selectRow(t, m, tc.row)
			m = keys(t, m, runes("s")...)
			tc.change(t, m)
			m = look(t, m)
			m = keys(t, m, runes("sub")...)
			viewHas(t, m, tc.prompt+"sub")
			viewHas(t, m, "saves as "+tc.preview)
			m = keys(t, m, enter)
			got := lines(t, m, "todo")
			if !slices.Contains(got, tc.want) || !slices.Contains(got, tc.preview) {
				t.Errorf("todo, want %q and its subtask %q:\n  %s (status %q)", tc.want, tc.preview, strings.Join(got, "\n  "), m.status)
			}
			if m.input != inputClosed {
				t.Error("the input is still open")
			}
		})
	}
}

// An edit open across a reload stays on its task too: its prompt gives
// the task's number now. When a reload finds the task changed or gone,
// the status line says so, the input stays open with what was typed
// (the next keys still go to it), its preview says it can't be saved,
// and Enter saves nothing, closing it, as for the other actions.
func TestWatch_AnEditFollowsItsTaskThroughAReload(t *testing.T) {
	m := fileTreeModel(t, launch...)
	m = selectRow(t, m, 4) // Pack the van
	m = keys(t, m, runes("e")...)
	edited := "(A) Zed\n" + strings.Join(launch, "\n") + "\n"
	if err := os.WriteFile(m.repo.ContextPath("todo"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	m = look(t, m)
	m = keys(t, m, runes("!")...)
	viewHas(t, m, "Edit task 5 > Pack the van!")

	elsewhere(t, m, func(ctx *domain.Context) error {
		_, err := ctx.Replace("5", mustTask(t, "Pack the van +changed"))
		return err
	})
	m = look(t, m)
	if !strings.Contains(m.status, "changed") {
		t.Errorf("status %q doesn't say the task changed", m.status)
	}
	m = keys(t, m, runes("?")...)
	if m.input != inputEdit {
		t.Fatal("the input closed under the typing")
	}
	viewHas(t, m, "Pack the van!?")
	viewHas(t, m, "can't save: task 5 was changed or removed elsewhere")
	before := lines(t, m, "todo")
	m = keys(t, m, enter)
	if got := lines(t, m, "todo"); !slices.Equal(got, before) {
		t.Errorf("todo changed: %q", got)
	}
	if m.input != inputClosed || !strings.Contains(m.status, "changed") {
		t.Errorf("input %v, status %q", m.input, m.status)
	}
}

// The question about open subtasks, waiting across a reload, gives the
// task's number now, and the answer completes it.
func TestWatch_TheQuestionFollowsItsTaskThroughAReload(t *testing.T) {
	m := fileTreeModel(t, launch...)
	m = keys(t, m, runes("x")...)
	viewHas(t, m, "Task 0 has 3 open subtasks.")
	edited := "(A) Zed\n" + strings.Join(launch, "\n") + "\n"
	if err := os.WriteFile(m.repo.ContextPath("todo"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	m = look(t, m)
	viewHas(t, m, "Task 1 has 3 open subtasks.")
	m = keys(t, m, runes("A")...)
	if got := lines(t, m, "todo"); !slices.Equal(got, []string{"(A) Zed", "Pack the van"}) {
		t.Errorf("todo: %q", got)
	}
	viewHas(t, m, "Completed task 1 and 3 open subtasks.")
}

// A search applied across a reload stays applied to the list reloaded,
// its rows numbered afresh, the cursor on the task it was on: an action
// then acts on that task.
func TestWatch_ASearchFollowsItsTasksThroughAReload(t *testing.T) {
	m := fileTreeModel(t, "alpha", "beta milk", "gamma", "delta milk")
	m = search(t, m, "milk")
	m = settle(t, m, keyPress("j")) // delta milk, task 3
	edited := "(A) zed milk\nalpha\nbeta milk\ngamma\ndelta milk\n"
	if err := os.WriteFile(m.repo.ContextPath("todo"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	m = look(t, m)
	if !m.list.IsFiltered() || !slices.Equal(shown(m), []string{"zed milk", "beta milk", "delta milk"}) {
		t.Fatalf("the search shows %q", shown(m))
	}
	if it, ok := m.selectedTask(); !ok || it.index != 4 || it.task.Raw != "delta milk" {
		t.Fatalf("selected %v, want delta milk, task 4 now", it)
	}
	m = keys(t, m, keyPress("d"))
	if got := lines(t, m, "todo"); !slices.Equal(got, []string{"(A) zed milk", "alpha", "beta milk", "gamma"}) {
		t.Errorf("todo: %q", got)
	}
}

// The watch looks again each second, from the start, for a list with a
// file; a list without one (the memory repository) isn't watched.
func TestWatch_LooksEachSecond(t *testing.T) {
	m := fileTreeModel(t, "one")
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init starts no watch")
	}
	got := make(chan tea.Msg, 1)
	began := time.Now()
	go func() { got <- cmd() }()
	select {
	case msg := <-got:
		if _, ok := msg.(watchMsg); !ok {
			t.Errorf("Init's command gave %T, want a look", msg)
		}
		if took := time.Since(began); took > 1500*time.Millisecond {
			t.Errorf("the first look came after %v, want about a second", took)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no look within five seconds")
	}

	memory, _ := seedModel(t, "todo", "one")
	if _, cmd := memory.Update(watchMsg{}); cmd != nil {
		t.Error("a list without a file is watched")
	}
}

// loadCounting counts the contexts loaded through it.
type loadCounting struct {
	domain.Repository
	loads int
}

func (r *loadCounting) Context(name string) (*domain.Context, error) {
	r.loads++
	return r.Repository.Context(name)
}

func (r *loadCounting) CurrentContext() (*domain.Context, error) {
	r.loads++
	return r.Repository.CurrentContext()
}

// The TUI's own write isn't a change to reload for: after an action,
// the next look finds the file as the action left it, and reads nothing.
// It used to read and parse the file again on the next tick, after
// every action.
func TestWatch_TheTUIsOwnWriteIsntReloaded(t *testing.T) {
	for _, key := range []string{"+", "x", "d"} {
		t.Run(key, func(t *testing.T) {
			m := fileTreeModel(t, "one", "two", "three")
			counting := &loadCounting{Repository: m.repo}
			m.repo = counting
			m = selectRow(t, m, 1)
			m = keys(t, m, runes(key)...)
			counting.loads = 0
			m = look(t, m)
			if counting.loads > 0 {
				t.Errorf("the look after %s loaded the list %d times, want none", key, counting.loads)
			}
		})
	}
}

// busy is a repository another htt holds: every change gives ErrBusy.
type busy struct{ *storage.MemoryRepository }

func (busy) Transact(func(domain.Repository) error) error { return domain.ErrBusy }

// SetCurrent is a change too.
func (busy) SetCurrent(string) error { return domain.ErrBusy }

// Context loads the named context wired to change through r.
func (r busy) Context(name string) (*domain.Context, error) {
	loaded, err := r.MemoryRepository.Context(name)
	if err != nil {
		return nil, err
	}
	ctx := domain.NewContext(r, name)
	ctx.Tasks = loaded.Tasks
	return ctx, nil
}

// Another htt busy with the files: the status line says so, the TUI
// stays open, and nothing changes; a tab switch stays on its tab. A tab
// switch used to quit the TUI, saying nothing.
func TestOnShown_AnotherHttBusySaysSoAndStaysOpen(t *testing.T) {
	m, repo := seedModel(t, "todo", "one", "two")
	if err := repo.Save(&domain.Context{Name: "work", Tasks: []*domain.Task{mustTask(t, "in work")}}); err != nil {
		t.Fatal(err)
	}
	ctx, err := busy{repo}.Context("todo") // the list shown changes through busy
	if err != nil {
		t.Fatal(err)
	}
	m = Model(busy{repo}, ctx, Options{})
	m.list.Select(1)
	for _, key := range []string{"+", "x", "d", "l", "h", "tab"} {
		next, cmd := m.Update(keyPress(key))
		if isQuit(cmd) {
			t.Errorf("%s: the TUI quit", key)
		}
		got := asModel(t, next)
		if got.status != "Another htt is busy with your files; try again." {
			t.Errorf("%s: status %q", key, got.status)
		}
		if got.context.Name != "todo" || got.contexts[got.contextCursor].Name != "todo" {
			t.Errorf("%s: the TUI shows %s, its tab %s", key, got.context.Name, got.contexts[got.contextCursor].Name)
		}
	}
	if got := stored(t, repo, "todo"); !slices.Equal(got, []string{"one", "two"}) {
		t.Errorf("todo = %q", got)
	}
	if name, _ := repo.CurrentContextName(); name != "todo" {
		t.Errorf("current = %s", name)
	}
}

// The TUI's actions racing other htt processes adding tasks (here
// goroutines, each with a repository of its own, as a process has):
// every task added lands, and so does every action.
func TestOnShown_ActionsRacingAddsLoseNothing(t *testing.T) {
	m := fileTreeModel(t, "one", "two", "Pack the van")
	m = selectRow(t, m, 2)
	dir := filepath.Dir(m.repo.ContextPath("todo"))

	const adders, each = 3, 15
	errs := make(chan error, adders)
	for a := range adders {
		go func() {
			r := storage.NewFileRepository(dir, dir)
			for i := range each {
				ctx, err := r.Context("todo")
				if err == nil {
					var task *domain.Task
					if task, err = domain.NewTask(fmt.Sprintf("2026-01-01 added %d-%d", a, i)); err == nil {
						err = ctx.AddTask(task)
					}
				}
				if err != nil {
					errs <- err
					return
				}
			}
			errs <- nil
		}()
	}
	const presses = 30
	for p := range presses {
		key := "+"
		if p%2 == 1 {
			key = "-"
		}
		m = keys(t, m, keyPress(key))
		if it, ok := m.selectedTask(); !ok || !strings.HasSuffix(it.task.Raw, "Pack the van") {
			t.Fatalf("press %d: the selection left Pack the van (status %q)", p, m.status)
		}
	}
	for range adders {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}

	got := lines(t, m, "todo")
	for a := range adders {
		for i := range each {
			want := fmt.Sprintf("2026-01-01 added %d-%d", a, i)
			if !slices.Contains(got, want) {
				t.Errorf("%q is lost", want)
			}
		}
	}
	if !slices.Contains(got, "Pack the van") || len(got) != 3+adders*each {
		t.Errorf("todo has %d lines, want %d, Pack the van among them without a priority:\n  %s", len(got), 3+adders*each, strings.Join(got, "\n  "))
	}
}
