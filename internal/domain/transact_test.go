package domain_test

// The domain's operations as transactions (docs/design/safe-writes.md):
// each runs on the context as the store holds it when it runs, runs
// again when the store changes underneath before it's saved, and
// leaves the caller's context holding the result. The memory
// repository's Underneath changes the store between a run and its
// check, as another program would.

import (
	"errors"
	"testing"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/storage"
)

// underneathOnce has repo's next transaction meet change, made in the
// store between its first run and its check; it returns how many runs
// there were.
func underneathOnce(t *testing.T, repo *storage.MemoryRepository, change func()) *int {
	t.Helper()
	runs := 0
	repo.Underneath = func(run int) {
		runs = run
		if run == 1 {
			repo.Underneath = nil // the change itself saves
			change()
			repo.Underneath = func(run int) { runs = run }
		}
	}
	return &runs
}

// addTo adds raw to the stored context name, as another htt would.
func addTo(t *testing.T, repo *storage.MemoryRepository, name, raw string) {
	t.Helper()
	if err := loadCtx(t, repo, name).AddTask(mustTask(t, raw)); err != nil {
		t.Fatal(err)
	}
}

func TestTransact_AddingRunsAgainOnATaskAddedUnderneath(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "todo", "first")
	ctx := loadCtx(t, repo, "todo")
	runs := underneathOnce(t, repo, func() { addTo(t, repo, "todo", "2026-01-01 added underneath") })

	if err := ctx.AddTask(mustTask(t, "2026-01-01 mine")); err != nil {
		t.Fatal(err)
	}
	if *runs != 2 {
		t.Errorf("ran %d times, want 2", *runs)
	}
	want := []string{"first", "2026-01-01 added underneath", "2026-01-01 mine"}
	wantLines(t, "stored", rawsOf(t, repo, "todo"), want...)
	wantLines(t, "the caller's context", rawsOfTasks(ctx.Tasks), want...)
}

// A completion run again moves the task to the done file once, and
// keeps what was added to both files underneath.
func TestTransact_CompletingRunsAgainKeepingBothFilesChanges(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "todo", "finish me", "stay")
	seed(t, repo, domain.DoneContextName, "x 2026-01-01 old")
	ctx := loadCtx(t, repo, "todo")
	runs := underneathOnce(t, repo, func() {
		addTo(t, repo, "todo", "2026-01-01 new here")
		seed(t, repo, domain.DoneContextName, "x 2026-01-01 old", "x 2026-01-02 done elsewhere")
	})

	done, err := ctx.Complete("0", domain.SubtasksAsk)
	if err != nil {
		t.Fatal(err)
	}
	if *runs != 2 {
		t.Errorf("ran %d times, want 2", *runs)
	}
	if len(done) != 1 || done[0].Entry() != "finish me" {
		t.Errorf("completed %v", rawsOfTasks(done))
	}
	wantLines(t, "todo", rawsOf(t, repo, "todo"), "stay", "2026-01-01 new here")
	got := rawsOf(t, repo, domain.DoneContextName)
	if len(got) != 3 || got[0] != "x 2026-01-01 old" || got[1] != "x 2026-01-02 done elsewhere" {
		t.Errorf("done: %q, want the old line, the one done elsewhere, then this", got)
	}
	wantLines(t, "the caller's context", rawsOfTasks(ctx.Tasks), "stay", "2026-01-01 new here")
}

// A task number is read when the operation runs: task 0 is the first
// task in the store then, not the one the caller loaded first
// (docs/design/safe-writes.md, "Limits").
func TestTransact_ATaskNumberIsReadWhenTheOperationRuns(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "todo", "(B) was first")
	ctx := loadCtx(t, repo, "todo")
	seed(t, repo, "todo", "(A) now first", "(B) was first")

	before, _, err := ctx.SetPriority("0", "C")
	if err != nil {
		t.Fatal(err)
	}
	if before.Raw != "(A) now first" {
		t.Errorf("changed %q, want the task first in the store", before.Raw)
	}
	wantLines(t, "todo", rawsOf(t, repo, "todo"), "(B) was first", "(C) now first")
}

// A store that changes under every run: the operation gives up, with
// nothing saved, after as many runs as the file repository makes.
func TestTransact_AStoreThatKeepsChangingSavesNothing(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "todo", "finish me")
	ctx := loadCtx(t, repo, "todo")
	runs := 0
	repo.Underneath = func(run int) {
		runs = run
		// Saved straight to the store, as another program writes.
		seed(t, repo, "todo", "finish me", "changed "+itoa(run))
	}

	_, err := ctx.Complete("0", domain.SubtasksAsk)
	if !errors.Is(err, domain.ErrKeepsChanging) {
		t.Fatalf("err = %v, want ErrKeepsChanging", err)
	}
	if runs != 5 {
		t.Errorf("ran %d times, want 5", runs)
	}
	if got := rawsOf(t, repo, domain.DoneContextName); len(got) != 0 {
		t.Errorf("done: %q, want nothing saved", got)
	}
	wantLines(t, "todo", rawsOf(t, repo, "todo"), "finish me", "changed 5")
}

// Run again, a subtask is added as if for the first time: its p:, and
// the id its parent gets, are those of the store as it is then.
func TestTransact_ASubtaskRunAgainTakesTheNewIDs(t *testing.T) {
	for _, c := range []struct {
		other, id string
	}{
		{"other id:1", "2"}, // 1 is taken now
		{"other", "1"},      // still free: the first run's p:1 doesn't count
	} {
		repo := newRepo(t)
		seed(t, repo, "todo", "parent")
		ctx := loadCtx(t, repo, "todo")
		runs := underneathOnce(t, repo, func() { seed(t, repo, "todo", "parent", c.other) })

		sub := mustTask(t, "2026-01-01 sub")
		if err := ctx.AddSubtask("0", sub); err != nil {
			t.Fatal(err)
		}
		if *runs != 2 {
			t.Errorf("ran %d times, want 2", *runs)
		}
		wantLines(t, "todo with "+c.other, rawsOf(t, repo, "todo"), "parent id:"+c.id, "2026-01-01 sub p:"+c.id, c.other)
		if sub.Raw != "2026-01-01 sub p:"+c.id {
			t.Errorf("the task added is %q", sub.Raw)
		}
	}
}

// When nothing changed underneath, the caller's own tasks are the ones
// changed: a task it holds shows the change, as it did before
// transactions (the CLI prints a parent given an id this way).
func TestTransact_TheCallersTasksSeeTheChange(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "todo", "parent")
	ctx := loadCtx(t, repo, "todo")
	parent := ctx.Tasks[0]
	if err := ctx.AddSubtask("0", mustTask(t, "2026-01-01 sub")); err != nil {
		t.Fatal(err)
	}
	if parent.Raw != "parent id:1" {
		t.Errorf("the parent the caller holds: %q, want it given its id", parent.Raw)
	}
}

// Several steps in one transaction (Context.Transact): what the body's
// operations save is saved together, once, and run again together.
func TestContext_TransactRunsStepsAsOneTransaction(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "todo", "one", "two")
	ctx := loadCtx(t, repo, "todo")
	runs := underneathOnce(t, repo, func() { addTo(t, repo, "todo", "2026-01-01 three") })

	bodies := 0
	err := ctx.Transact(func(c *domain.Context) error {
		bodies++
		if _, _, err := c.SetPriority("0", "A"); err != nil {
			return err
		}
		_, err := c.Complete(itoa(len(c.Tasks)-1), domain.SubtasksAsk)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if bodies != 2 || *runs != 2 {
		t.Errorf("the body ran %d times in %d runs, want 2 and 2", bodies, *runs)
	}
	wantLines(t, "todo", rawsOf(t, repo, "todo"), "(A) one", "two")
	if got := rawsOf(t, repo, domain.DoneContextName); len(got) != 1 {
		t.Errorf("done: %q, want the task added underneath, completed", got)
	}
}

// An error from the body saves nothing, though steps before it did.
func TestContext_TransactSavesNothingOnAnError(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "todo", "one")
	ctx := loadCtx(t, repo, "todo")
	refused := errors.New("refused")
	err := ctx.Transact(func(c *domain.Context) error {
		if err := c.AddTask(mustTask(t, "2026-01-01 two")); err != nil {
			return err
		}
		return refused
	})
	if err != refused {
		t.Fatalf("err = %v, want the body's", err)
	}
	wantLines(t, "todo", rawsOf(t, repo, "todo"), "one")
}

func itoa(i int) string {
	return string(rune('0' + i))
}
