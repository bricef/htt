package domain_test

// The domain's operations as transactions (docs/design/safe-writes.md):
// each runs on the context as the store holds it when it runs, runs
// again when the store changes underneath before it's saved, and
// leaves the caller's context holding the result. The memory
// repository's Underneath changes the store between a run and its
// check, as another program would.

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

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

// Logging an entry runs again on an entry logged underneath, keeping
// both, in time order with AppendAt.
func TestTransact_LoggingRunsAgainOnAnEntryLoggedUnderneath(t *testing.T) {
	logs := storage.NewMemoryTimelogRepository()
	now := time.Now()
	day, err := logs.Day(now)
	if err != nil {
		t.Fatal(err)
	}
	runs := 0
	logs.Underneath = func(run int) {
		runs = run
		if run == 1 {
			other, _ := logs.Day(now)
			other.Entries = append(other.Entries, mustTask(t, "elsewhere ts:"+now.Add(-time.Minute).UTC().Format(time.RFC3339)))
			if err := logs.Save(other); err != nil { // straight to the store
				t.Fatal(err)
			}
		}
	}
	if err := day.Append(mustTask(t, "mine")); err != nil {
		t.Fatal(err)
	}
	if runs != 2 {
		t.Errorf("ran %d times, want 2", runs)
	}
	stored, _ := logs.Day(now)
	if got := rawsOfTasks(stored.Entries); len(got) != 2 || !strings.HasPrefix(got[0], "elsewhere ") || !strings.HasPrefix(got[1], "mine ts:") {
		t.Errorf("the log: %q, want both entries", got)
	}
	if got := rawsOfTasks(day.Entries); len(got) != 2 {
		t.Errorf("the caller's log: %q, want both entries", got)
	}

	// AppendAt places by time in the log as it is when it runs.
	logs.Underneath = func(run int) {
		if run == 1 {
			other, _ := logs.Day(now)
			other.Entries = append([]*domain.Task{mustTask(t, "earliest ts:"+now.Add(-time.Hour).UTC().Format(time.RFC3339))}, other.Entries...)
			if err := logs.Save(other); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := day.AppendAt(mustTask(t, "between"), now.Add(-30*time.Minute), now); err != nil {
		t.Fatal(err)
	}
	stored, _ = logs.Day(now)
	var texts []string
	for _, e := range stored.Entries {
		texts = append(texts, strings.Fields(e.Raw)[0])
	}
	if !slices.Equal(texts, []string{"earliest", "between", "elsewhere", "mine"}) {
		t.Errorf("the log: %q", rawsOfTasks(stored.Entries))
	}
}

func itoa(i int) string {
	return string(rune('0' + i))
}

// loadCounter is a repository whose transactions count the contexts
// they load (parse), by name, and append to a context as a file
// repository does (domain.Appender), without loading it.
type loadCounter struct {
	*storage.MemoryRepository
	loads map[string]int
}

func (r *loadCounter) Transact(change func(domain.Repository) error) error {
	return r.MemoryRepository.Transact(func(tx domain.Repository) error {
		return change(countingTx{tx, r})
	})
}

// Context loads the named context wired to transact through r.
func (r *loadCounter) Context(name string) (*domain.Context, error) {
	loaded, err := r.MemoryRepository.Context(name)
	if err != nil {
		return nil, err
	}
	ctx := domain.NewContext(r, name)
	ctx.Tasks = loaded.Tasks
	return ctx, nil
}

type countingTx struct {
	domain.Repository
	r *loadCounter
}

func (tx countingTx) Context(name string) (*domain.Context, error) {
	tx.r.loads[name]++
	return tx.Repository.Context(name)
}

func (tx countingTx) ContextLines(name string) ([]string, error) {
	return tx.Repository.(domain.LineSource).ContextLines(name)
}

func (tx countingTx) Append(name string, tasks []*domain.Task) error {
	ctx, err := tx.Repository.Context(name) // the store's, uncounted
	if err != nil {
		return err
	}
	ctx.Tasks = append(ctx.Tasks, tasks...)
	return tx.Save(ctx)
}

// While a transaction holds the store, an operation parses nothing it
// needn't: the context the caller loaded, unchanged since, isn't loaded
// again (its lines are compared, unparsed), and the done file and the
// archive are appended to, not loaded. Every command used to parse its
// context twice, the second time holding the lock, and a completion the
// whole done file too.
func TestTransact_AnOperationParsesNothingItNeedntUnderTheLock(t *testing.T) {
	for _, op := range []struct {
		name   string
		run    func(ctx *domain.Context) error
		target string
		want   string
	}{
		{"add", func(ctx *domain.Context) error { return ctx.AddTask(mustTask(t, "2026-01-01 new")) }, "todo", "2026-01-01 new"},
		{"complete", func(ctx *domain.Context) error { _, err := ctx.Complete("0", domain.SubtasksAsk); return err }, domain.DoneContextName, " first context:todo"},
		{"delete", func(ctx *domain.Context) error { _, err := ctx.Delete("0"); return err }, domain.ArchiveContextName, "first archived-from:todo"},
		{"move to done", func(ctx *domain.Context) error { _, err := ctx.Move("0", domain.DoneContextName); return err }, domain.DoneContextName, "first"},
		{"raise", func(ctx *domain.Context) error { _, _, err := ctx.IncreasePriority("1"); return err }, "todo", "(C) second"},
	} {
		t.Run(op.name, func(t *testing.T) {
			repo := &loadCounter{MemoryRepository: newRepo(t), loads: map[string]int{}}
			seed(t, repo.MemoryRepository, "todo", "first", "second")
			seed(t, repo.MemoryRepository, domain.DoneContextName, "x 2026-01-01 old")
			seed(t, repo.MemoryRepository, domain.ArchiveContextName, "gone deleted-on:2026-01-01")
			ctx, err := repo.Context("todo")
			if err != nil {
				t.Fatal(err)
			}
			if err := op.run(ctx); err != nil {
				t.Fatal(err)
			}
			if len(repo.loads) > 0 {
				t.Errorf("loaded under the lock: %v", repo.loads)
			}
			if got := rawsOf(t, repo.MemoryRepository, op.target); !slices.ContainsFunc(got, func(l string) bool { return strings.Contains(l, op.want) }) {
				t.Errorf("%s: %q, want a line with %q", op.target, got, op.want)
			}
		})
	}

	// Changed since it was loaded (as when other htt processes add tasks
	// meanwhile, which all of them waiting for the lock do), the context
	// isn't loaded again either: the tasks it holds keep their parse, and
	// only the lines added are parsed.
	repo := &loadCounter{MemoryRepository: newRepo(t), loads: map[string]int{}}
	seed(t, repo.MemoryRepository, "todo", "first", "second")
	ctx, err := repo.Context("todo")
	if err != nil {
		t.Fatal(err)
	}
	first := ctx.Tasks[0]
	seed(t, repo.MemoryRepository, "todo", "(A) added elsewhere", "first")
	if err := ctx.AddTask(mustTask(t, "2026-01-01 new")); err != nil {
		t.Fatal(err)
	}
	if len(repo.loads) > 0 {
		t.Errorf("loaded under the lock: %v", repo.loads)
	}
	want := []string{"(A) added elsewhere", "first", "2026-01-01 new"}
	wantLines(t, "todo", rawsOf(t, repo.MemoryRepository, "todo"), want...)
	wantLines(t, "the caller's context", rawsOfTasks(ctx.Tasks), want...)
	if !slices.Contains(ctx.Tasks, first) {
		t.Error("the task the caller held for a line still there was parsed again")
	}
	if got := ctx.Tasks[0]; got.Priority != "A" || got.Line != 0 || got.FileLine != 1 {
		t.Errorf("the line added elsewhere parsed as %+v", got)
	}
}
