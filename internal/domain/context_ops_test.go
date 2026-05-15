package domain_test

// Tests for the persistent Context operations introduced in Step 3.
// Lives in the _test package (black-box) so it can import storage for a
// real Repository implementation. Pure-method tests stay in
// context_test.go (white-box, package domain).

import (
	"strings"
	"testing"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/storage"
)

func mustTask(t *testing.T, raw string) *domain.Task {
	t.Helper()
	task, err := domain.NewTask(raw)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", raw, err)
	}
	return task
}

// newRepo returns a fresh in-memory repo for one test.
func newRepo(t *testing.T) *storage.MemoryRepository {
	t.Helper()
	return storage.NewMemoryRepository()
}

// currentCtx wraps repo.CurrentContext() with a Fatal on error.
func currentCtx(t *testing.T, repo *storage.MemoryRepository) *domain.Context {
	t.Helper()
	ctx, err := repo.CurrentContext()
	if err != nil {
		t.Fatalf("CurrentContext: %v", err)
	}
	return ctx
}

// loadCtx wraps repo.Context(name).
func loadCtx(t *testing.T, repo *storage.MemoryRepository, name string) *domain.Context {
	t.Helper()
	ctx, err := repo.Context(name)
	if err != nil {
		t.Fatalf("Context(%q): %v", name, err)
	}
	return ctx
}

// --- AddTask ---------------------------------------------------------------

func TestContext_AddTask_AppendsAndPersists(t *testing.T) {
	repo := newRepo(t)
	ctx := currentCtx(t, repo)

	if err := ctx.AddTask(mustTask(t, "buy milk")); err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	if len(ctx.Tasks) != 1 || ctx.Tasks[0].Raw != "buy milk" {
		t.Errorf("in-memory ctx.Tasks = %v, want one buy-milk", ctx.Tasks)
	}

	stored := loadCtx(t, repo, "todo")
	if len(stored.Tasks) != 1 || stored.Tasks[0].Raw != "buy milk" {
		t.Errorf("persisted tasks = %v, want one buy-milk", stored.Tasks)
	}
}

func TestContext_AddTask_AppendsToNamedContextNotCurrent(t *testing.T) {
	repo := newRepo(t)
	work := loadCtx(t, repo, "work")
	if err := work.AddTask(mustTask(t, "ship feature")); err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	storedWork := loadCtx(t, repo, "work")
	if len(storedWork.Tasks) != 1 || storedWork.Tasks[0].Raw != "ship feature" {
		t.Errorf("work tasks = %v", storedWork.Tasks)
	}
	storedTodo := loadCtx(t, repo, "todo")
	if len(storedTodo.Tasks) != 0 {
		t.Errorf("todo should be empty, got %v", storedTodo.Tasks)
	}
}

// --- Delete ---------------------------------------------------------------

func TestContext_Delete_RemovesAndPersists(t *testing.T) {
	repo := newRepo(t)
	ctx := currentCtx(t, repo)
	_ = ctx.AddTask(mustTask(t, "keep"))
	_ = ctx.AddTask(mustTask(t, "delete me"))

	task, err := ctx.Delete("1")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if task.Raw != "delete me" {
		t.Errorf("deleted task = %q, want 'delete me'", task.Raw)
	}

	stored := loadCtx(t, repo, "todo")
	if len(stored.Tasks) != 1 || stored.Tasks[0].Raw != "keep" {
		t.Errorf("stored = %v, want one 'keep'", stored.Tasks)
	}
}

func TestContext_Delete_OutOfRangeErrors(t *testing.T) {
	repo := newRepo(t)
	ctx := currentCtx(t, repo)
	if _, err := ctx.Delete("99"); err == nil {
		t.Errorf("expected out-of-range error")
	}
}

// --- Replace --------------------------------------------------------------

func TestContext_Replace_SwapsAndPersists(t *testing.T) {
	repo := newRepo(t)
	ctx := currentCtx(t, repo)
	_ = ctx.AddTask(mustTask(t, "old"))

	old, err := ctx.Replace("0", mustTask(t, "new"))
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if old.Raw != "old" {
		t.Errorf("snapshot = %q, want 'old'", old.Raw)
	}

	stored := loadCtx(t, repo, "todo")
	if len(stored.Tasks) != 1 || stored.Tasks[0].Raw != "new" {
		t.Errorf("stored = %v, want one 'new'", stored.Tasks)
	}
}

func TestContext_Replace_TaskNotFoundErrors(t *testing.T) {
	repo := newRepo(t)
	ctx := currentCtx(t, repo)
	_ = ctx.AddTask(mustTask(t, "only"))
	if _, err := ctx.Replace("99", mustTask(t, "noop")); err == nil {
		t.Errorf("expected out-of-range error")
	}
}

// --- Move -----------------------------------------------------------------

func TestContext_Move_TransfersBetweenContexts(t *testing.T) {
	repo := newRepo(t)
	ctx := currentCtx(t, repo)
	_ = ctx.AddTask(mustTask(t, "moveme"))

	task, err := ctx.Move("0", "work")
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if task.Raw != "moveme" {
		t.Errorf("moved task = %q, want 'moveme'", task.Raw)
	}

	source := loadCtx(t, repo, "todo")
	if len(source.Tasks) != 0 {
		t.Errorf("source should be empty, got %v", source.Tasks)
	}
	dest := loadCtx(t, repo, "work")
	if len(dest.Tasks) != 1 || dest.Tasks[0].Raw != "moveme" {
		t.Errorf("dest = %v, want one 'moveme'", dest.Tasks)
	}
}

// --- Complete -------------------------------------------------------------

func TestContext_Complete_MovesToDoneAndAnnotates(t *testing.T) {
	repo := newRepo(t)
	ctx := currentCtx(t, repo)
	_ = ctx.AddTask(mustTask(t, "make tea"))

	task, err := ctx.Complete("0")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !task.Completed {
		t.Errorf("task should be marked Completed")
	}
	if task.Annotations["context"] != "todo" {
		t.Errorf("Annotations[context] = %q, want todo", task.Annotations["context"])
	}

	todo := loadCtx(t, repo, "todo")
	if len(todo.Tasks) != 0 {
		t.Errorf("todo should be empty after complete, got %v", todo.Tasks)
	}
	done := loadCtx(t, repo, "done")
	if len(done.Tasks) != 1 || !strings.Contains(done.Tasks[0].Raw, "make tea") {
		t.Errorf("done should contain completed task, got %v", done.Tasks)
	}
	if !strings.HasPrefix(done.Tasks[0].Raw, "x ") {
		t.Errorf("completed task should start with 'x ', got %q", done.Tasks[0].Raw)
	}
}

func TestContext_Complete_OutOfRangeErrors(t *testing.T) {
	repo := newRepo(t)
	ctx := currentCtx(t, repo)
	if _, err := ctx.Complete("99"); err == nil {
		t.Errorf("expected out-of-range error")
	}
}

// --- Priority methods -----------------------------------------------------

func TestContext_SetPriority_RewritesTaskAndPersists(t *testing.T) {
	repo := newRepo(t)
	ctx := currentCtx(t, repo)
	_ = ctx.AddTask(mustTask(t, "urgent thing"))

	_, neu, err := ctx.SetPriority("0", "A")
	if err != nil {
		t.Fatalf("SetPriority: %v", err)
	}
	if neu.Priority != "A" {
		t.Errorf("priority = %q, want A", neu.Priority)
	}
	stored := loadCtx(t, repo, "todo")
	if !strings.HasPrefix(stored.Tasks[0].Raw, "(A) ") {
		t.Errorf("stored = %q, want (A) prefix", stored.Tasks[0].Raw)
	}
}

func TestContext_SetPriority_InvalidErrors(t *testing.T) {
	repo := newRepo(t)
	ctx := currentCtx(t, repo)
	_ = ctx.AddTask(mustTask(t, "x"))
	if _, _, err := ctx.SetPriority("0", "Z"); err == nil {
		t.Errorf("expected error on invalid priority")
	}
}

func TestContext_SetPriority_SnapshotPreservesPreMutationView(t *testing.T) {
	// The snapshot/mutated pair is the API guarantee that lets CLI print
	// "Before:" and "After:" lines. If the implementation slipped to
	// returning two references to the same (post-mutation) object, the
	// "Before:" line would lie.
	repo := newRepo(t)
	ctx := currentCtx(t, repo)
	_ = ctx.AddTask(mustTask(t, "(C) something"))

	old, neu, err := ctx.IncreasePriority("0")
	if err != nil {
		t.Fatalf("IncreasePriority: %v", err)
	}
	if old.Priority != "C" {
		t.Errorf("snapshot Priority = %q, want C", old.Priority)
	}
	if neu.Priority != "B" {
		t.Errorf("mutated Priority = %q, want B", neu.Priority)
	}
	if old.Raw == neu.Raw {
		t.Errorf("old and new Raw should differ; both are %q", old.Raw)
	}
}

func TestContext_IncreasePriority_StepsUpAndPersists(t *testing.T) {
	repo := newRepo(t)
	ctx := currentCtx(t, repo)
	_ = ctx.AddTask(mustTask(t, "(C) thing"))

	_, neu, err := ctx.IncreasePriority("0")
	if err != nil {
		t.Fatalf("IncreasePriority: %v", err)
	}
	if neu.Priority != "B" {
		t.Errorf("priority = %q, want B", neu.Priority)
	}
	stored := loadCtx(t, repo, "todo")
	if !strings.HasPrefix(stored.Tasks[0].Raw, "(B) ") {
		t.Errorf("stored = %q, want (B) prefix", stored.Tasks[0].Raw)
	}
}

func TestContext_DecreasePriority_StepsDownAndPersists(t *testing.T) {
	repo := newRepo(t)
	ctx := currentCtx(t, repo)
	_ = ctx.AddTask(mustTask(t, "(A) thing"))

	_, neu, err := ctx.DecreasePriority("0")
	if err != nil {
		t.Fatalf("DecreasePriority: %v", err)
	}
	if neu.Priority != "B" {
		t.Errorf("priority = %q, want B", neu.Priority)
	}
	stored := loadCtx(t, repo, "todo")
	if !strings.HasPrefix(stored.Tasks[0].Raw, "(B) ") {
		t.Errorf("stored = %q, want (B) prefix", stored.Tasks[0].Raw)
	}
}

// --- Cross-test: repo wiring observable through behavior -----------------

func TestContext_PersistentMethodsSaveThroughInjectedRepo(t *testing.T) {
	// The Step 2 wiring invariant (TestNewContext_InjectsRepo) checks the
	// field exists. This test verifies it's actually used: a fresh ctx
	// from repo.Context, an AddTask, then a fresh load from the same
	// repo, should round-trip — which can only happen if the Context's
	// repo field matches the repo we built it from.
	repo := newRepo(t)
	ctx := loadCtx(t, repo, "alpha")

	if err := ctx.AddTask(mustTask(t, "anchored")); err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	fresh := loadCtx(t, repo, "alpha")
	if len(fresh.Tasks) != 1 || fresh.Tasks[0].Raw != "anchored" {
		t.Errorf("repo round-trip failed; got %v", fresh.Tasks)
	}
}
