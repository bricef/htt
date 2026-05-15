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

// --- Bug-fix regression tests --------------------------------------------

func TestContext_Move_SameContextRejected(t *testing.T) {
	// bug_002: Moving to the same context used to load the source twice
	// and produce duplication (the destination load was a snapshot taken
	// before remove). We now reject the call up front so the duplication
	// path is unreachable.
	repo := newRepo(t)
	ctx := currentCtx(t, repo)
	_ = ctx.AddTask(mustTask(t, "stay"))

	if _, err := ctx.Move("0", ctx.Name); err == nil {
		t.Errorf("Move to same context should error")
	}

	stored := loadCtx(t, repo, "todo")
	if len(stored.Tasks) != 1 || stored.Tasks[0].Raw != "stay" {
		t.Errorf("stored = %v, want one 'stay' (no duplication)", stored.Tasks)
	}
}

func TestContext_Complete_FromDoneContextRejected(t *testing.T) {
	// bug_002 sibling: Complete from the done context would re-mark and
	// duplicate via the same load-twice mechanism. Reject up front.
	repo := newRepo(t)
	done := loadCtx(t, repo, domain.DoneContextName)
	_ = done.AddTask(mustTask(t, "already done"))

	if _, err := done.Complete("0"); err == nil {
		t.Errorf("Complete from done context should error")
	}

	stored := loadCtx(t, repo, domain.DoneContextName)
	if len(stored.Tasks) != 1 {
		t.Errorf("done should still hold one task, got %v", stored.Tasks)
	}
}

func TestContext_SetPriority_RejectsLettersOutsideABC(t *testing.T) {
	// bug_007: The old regex was [A-F], but Task.setPriority only knows
	// A, B, C. Letters D, E, F passed validation and were silently
	// coerced to empty priority — a typo erased the user's priority
	// with no diagnostic. The tightened regex rejects them.
	repo := newRepo(t)
	ctx := currentCtx(t, repo)
	_ = ctx.AddTask(mustTask(t, "(A) thing"))

	for _, bad := range []string{"D", "E", "F", "a", "0", ""} {
		if _, _, err := ctx.SetPriority("0", bad); err == nil {
			t.Errorf("SetPriority(%q) should error, got nil", bad)
		}
	}

	// And the task should still hold its (A) priority — no silent strip.
	stored := loadCtx(t, repo, "todo")
	if stored.Tasks[0].Priority != "A" {
		t.Errorf("Priority = %q, want A (rejected calls should not mutate)", stored.Tasks[0].Priority)
	}
}

func TestContext_IncreasePriority_PersistsSortedOrder(t *testing.T) {
	// bug_001: Sort used to happen only in-memory in the TUI, after the
	// repo Save. The on-disk file kept the pre-sort order, so a refresh
	// (context switch, add, delete) showed unsorted tasks — display
	// would flicker from sorted (post-keystroke) to unsorted (post-refresh).
	// Sort now runs before Save so the disk file matches the displayed view.
	repo := newRepo(t)
	ctx := currentCtx(t, repo)
	_ = ctx.AddTask(mustTask(t, "(C) low"))
	_ = ctx.AddTask(mustTask(t, "no pri"))
	_ = ctx.AddTask(mustTask(t, "(A) urgent"))

	// Bump "no pri" up — it becomes (C). After Sort the on-disk order
	// should be A first, the two Cs after in stable order.
	if _, _, err := ctx.IncreasePriority("1"); err != nil {
		t.Fatalf("IncreasePriority: %v", err)
	}

	stored := loadCtx(t, repo, "todo")
	wantOrder := []string{"(A) urgent", "(C) low", "(C) no pri"}
	if len(stored.Tasks) != len(wantOrder) {
		t.Fatalf("len(stored.Tasks) = %d, want %d", len(stored.Tasks), len(wantOrder))
	}
	for i, want := range wantOrder {
		if stored.Tasks[i].Raw != want {
			t.Errorf("Tasks[%d] = %q, want %q (disk order should match displayed sort)",
				i, stored.Tasks[i].Raw, want)
		}
	}
}

func TestContext_Move_SavesDestinationFirst(t *testing.T) {
	// bug_015: A partial-save failure between the two Saves (e.g. ENOSPC
	// on the second one) is recoverable only when destination is saved
	// first. Source-first: a failure on Save#2 leaves the source empty
	// and the destination empty — task lost. Destination-first: a
	// failure on Save#2 leaves both holding the task — recoverable.
	repo := newOrderTrackingRepo()
	if err := repo.SetCurrent("todo"); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}
	repo.seed("todo", mustTask(t, "moveme"))

	ctx, _ := repo.Context("todo")
	if _, err := ctx.Move("0", "work"); err != nil {
		t.Fatalf("Move: %v", err)
	}

	if len(repo.saved) != 2 {
		t.Fatalf("expected 2 Save calls, got %d (%v)", len(repo.saved), repo.saved)
	}
	if repo.saved[0] != "work" {
		t.Errorf("first Save should be destination 'work', got order %v", repo.saved)
	}
}

func TestContext_Complete_SavesDoneFirst(t *testing.T) {
	// bug_015 sibling: same partial-failure reasoning for Complete.
	repo := newOrderTrackingRepo()
	if err := repo.SetCurrent("todo"); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}
	repo.seed("todo", mustTask(t, "finishme"))

	ctx, _ := repo.Context("todo")
	if _, err := ctx.Complete("0"); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if len(repo.saved) != 2 {
		t.Fatalf("expected 2 Save calls, got %d", len(repo.saved))
	}
	if repo.saved[0] != domain.DoneContextName {
		t.Errorf("first Save should be 'done', got order %v", repo.saved)
	}
}

// --- Cross-test: repo wiring observable through behavior -----------------

// orderTrackingRepo records the order in which Save was called for each
// context name. Used to pin the destination-first invariant from
// bug_015. Wrapping storage.MemoryRepository via struct embedding does
// NOT work for this: MemoryRepository.Context wires the returned
// Context to the *MemoryRepository receiver, so saves bypass any
// overridden Save on the outer wrapper. We instead implement
// domain.Repository directly here and pass the wrapper to
// domain.NewContext so c.repo points at us.
type orderTrackingRepo struct {
	contexts map[string][]*domain.Task
	current  string
	saved    []string
}

func newOrderTrackingRepo() *orderTrackingRepo {
	return &orderTrackingRepo{contexts: map[string][]*domain.Task{}}
}

func (r *orderTrackingRepo) seed(name string, tasks ...*domain.Task) {
	cp := make([]*domain.Task, len(tasks))
	copy(cp, tasks)
	r.contexts[name] = cp
}

func (r *orderTrackingRepo) Context(name string) (*domain.Context, error) {
	ctx := domain.NewContext(r, name)
	stored := r.contexts[name]
	ctx.Tasks = make([]*domain.Task, len(stored))
	copy(ctx.Tasks, stored)
	return ctx, nil
}

func (r *orderTrackingRepo) Contexts() ([]*domain.Context, error) {
	out := make([]*domain.Context, 0, len(r.contexts))
	for name := range r.contexts {
		ctx, _ := r.Context(name)
		out = append(out, ctx)
	}
	return out, nil
}

func (r *orderTrackingRepo) ContextNames() ([]string, error) {
	out := make([]string, 0, len(r.contexts))
	for name := range r.contexts {
		out = append(out, name)
	}
	return out, nil
}

func (r *orderTrackingRepo) CurrentContext() (*domain.Context, error) {
	return r.Context(r.current)
}

func (r *orderTrackingRepo) CurrentContextName() (string, error) { return r.current, nil }
func (r *orderTrackingRepo) SetCurrent(name string) error        { r.current = name; return nil }

func (r *orderTrackingRepo) Save(c *domain.Context) error {
	r.saved = append(r.saved, c.Name)
	cp := make([]*domain.Task, len(c.Tasks))
	copy(cp, c.Tasks)
	r.contexts[c.Name] = cp
	return nil
}

func (r *orderTrackingRepo) ContextPath(string) string { return "" }

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
