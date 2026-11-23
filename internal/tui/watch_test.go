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
// stays open, and nothing changes.
func TestOnShown_AnotherHttBusySaysSoAndStaysOpen(t *testing.T) {
	m, repo := seedModel(t, "todo", "one", "two")
	m.repo = busy{repo}
	var err error
	if m.context, err = m.repo.Context("todo"); err != nil {
		t.Fatal(err)
	}
	m.list.Select(1)
	for _, key := range []string{"+", "x", "d"} {
		next, cmd := m.Update(keyPress(key))
		if isQuit(cmd) {
			t.Errorf("%s: the TUI quit", key)
		}
		if got := asModel(t, next).status; got != "Another htt is busy with your files; try again." {
			t.Errorf("%s: status %q", key, got)
		}
	}
	if got := stored(t, repo, "todo"); !slices.Equal(got, []string{"one", "two"}) {
		t.Errorf("todo = %q", got)
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
