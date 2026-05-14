package usecase

import (
	"strings"
	"testing"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/storage"
)

func newUC(t *testing.T) (*UseCases, *storage.MemoryRepository) {
	t.Helper()
	repo := storage.NewMemoryRepository()
	return New(repo), repo
}

func mustLoad(t *testing.T, repo *storage.MemoryRepository, name string) *domain.Context {
	t.Helper()
	ctx, err := repo.LoadContext(name)
	if err != nil {
		t.Fatalf("LoadContext(%q): %v", name, err)
	}
	return ctx
}

func TestAddTask_AppendsToCurrentContext(t *testing.T) {
	uc, repo := newUC(t)

	task, ctx, err := uc.AddTask("buy milk")
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	if task.Raw != "buy milk" {
		t.Errorf("task.Raw = %q, want buy milk", task.Raw)
	}
	if ctx.Name != "todo" {
		t.Errorf("ctx.Name = %q, want todo (default)", ctx.Name)
	}

	stored := mustLoad(t, repo, "todo")
	if len(stored.Tasks) != 1 || stored.Tasks[0].Raw != "buy milk" {
		t.Errorf("stored tasks = %v", stored.Tasks)
	}
}

func TestAddTask_EmptyRawErrors(t *testing.T) {
	uc, _ := newUC(t)
	if _, _, err := uc.AddTask(""); err == nil {
		t.Errorf("expected error on empty raw")
	}
}

func TestAddTaskTo_WritesNamedContext(t *testing.T) {
	uc, repo := newUC(t)

	_, _, err := uc.AddTaskTo("work", "ship feature")
	if err != nil {
		t.Fatalf("AddTaskTo: %v", err)
	}

	work := mustLoad(t, repo, "work")
	if len(work.Tasks) != 1 || work.Tasks[0].Raw != "ship feature" {
		t.Errorf("work tasks = %v", work.Tasks)
	}
	todo := mustLoad(t, repo, "todo")
	if len(todo.Tasks) != 0 {
		t.Errorf("todo should be empty, got %v", todo.Tasks)
	}
}

func TestCompleteTask_MovesToDone(t *testing.T) {
	uc, repo := newUC(t)
	_, _, _ = uc.AddTask("make tea")

	task, err := uc.CompleteTask("0")
	if err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}
	if !task.Completed {
		t.Errorf("task should be Completed")
	}
	if task.Annotations["context"] != "todo" {
		t.Errorf("Annotations[context] = %q, want todo", task.Annotations["context"])
	}

	todo := mustLoad(t, repo, "todo")
	if len(todo.Tasks) != 0 {
		t.Errorf("todo should be empty after complete, got %v", todo.Tasks)
	}
	done := mustLoad(t, repo, "done")
	if len(done.Tasks) != 1 || !strings.Contains(done.Tasks[0].Raw, "make tea") {
		t.Errorf("done should contain completed task, got %v", done.Tasks)
	}
	if !strings.HasPrefix(done.Tasks[0].Raw, "x ") {
		t.Errorf("completed task should start with 'x ', got %q", done.Tasks[0].Raw)
	}
}

func TestCompleteTask_OutOfRangeErrors(t *testing.T) {
	uc, _ := newUC(t)
	if _, err := uc.CompleteTask("99"); err == nil {
		t.Errorf("expected out-of-range error")
	}
}

func TestDeleteTask_RemovesFromCurrentContext(t *testing.T) {
	uc, repo := newUC(t)
	_, _, _ = uc.AddTask("keep")
	_, _, _ = uc.AddTask("delete me")

	task, err := uc.DeleteTask("1")
	if err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	if task.Raw != "delete me" {
		t.Errorf("got %q, want 'delete me'", task.Raw)
	}

	stored := mustLoad(t, repo, "todo")
	if len(stored.Tasks) != 1 || stored.Tasks[0].Raw != "keep" {
		t.Errorf("stored = %v", stored.Tasks)
	}
}

func TestMoveTask_BetweenContexts(t *testing.T) {
	uc, repo := newUC(t)
	_, _, _ = uc.AddTask("moveme")

	task, from, to, err := uc.MoveTask("0", "work")
	if err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	if task.Raw != "moveme" || from != "todo" || to != "work" {
		t.Errorf("got task=%q from=%q to=%q", task.Raw, from, to)
	}

	src := mustLoad(t, repo, "todo")
	if len(src.Tasks) != 0 {
		t.Errorf("source should be empty, got %v", src.Tasks)
	}
	dst := mustLoad(t, repo, "work")
	if len(dst.Tasks) != 1 || dst.Tasks[0].Raw != "moveme" {
		t.Errorf("dest = %v", dst.Tasks)
	}
}

func TestReplaceTask_SwapsRaw(t *testing.T) {
	uc, repo := newUC(t)
	_, _, _ = uc.AddTask("old")

	old, neu, err := uc.ReplaceTask("0", "new")
	if err != nil {
		t.Fatalf("ReplaceTask: %v", err)
	}
	if old.Raw != "old" || neu.Raw != "new" {
		t.Errorf("old=%q new=%q", old.Raw, neu.Raw)
	}

	stored := mustLoad(t, repo, "todo")
	if len(stored.Tasks) != 1 || stored.Tasks[0].Raw != "new" {
		t.Errorf("stored = %v", stored.Tasks)
	}
}

func TestReplaceTask_EmptyErrors(t *testing.T) {
	uc, _ := newUC(t)
	_, _, _ = uc.AddTask("a")
	if _, _, err := uc.ReplaceTask("0", ""); err == nil {
		t.Errorf("expected error on empty replacement")
	}
}

func TestSetPriority_RewritesTask(t *testing.T) {
	uc, repo := newUC(t)
	_, _, _ = uc.AddTask("urgent thing")

	_, neu, err := uc.SetPriority("0", "A")
	if err != nil {
		t.Fatalf("SetPriority: %v", err)
	}
	if neu.Priority != "A" {
		t.Errorf("priority = %q, want A", neu.Priority)
	}
	stored := mustLoad(t, repo, "todo")
	if !strings.HasPrefix(stored.Tasks[0].Raw, "(A) ") {
		t.Errorf("stored = %q, want (A) prefix", stored.Tasks[0].Raw)
	}
}

func TestSetPriority_InvalidPriorityErrors(t *testing.T) {
	uc, _ := newUC(t)
	_, _, _ = uc.AddTask("x")
	if _, _, err := uc.SetPriority("0", "Z"); err == nil {
		t.Errorf("expected error on invalid priority")
	}
}

func TestIncreasePriority_StepsUp(t *testing.T) {
	uc, repo := newUC(t)
	_, _, _ = uc.AddTask("(C) something")

	_, neu, err := uc.IncreasePriority("0")
	if err != nil {
		t.Fatalf("IncreasePriority: %v", err)
	}
	if neu.Priority != "B" {
		t.Errorf("priority = %q, want B", neu.Priority)
	}
	stored := mustLoad(t, repo, "todo")
	if !strings.HasPrefix(stored.Tasks[0].Raw, "(B) ") {
		t.Errorf("stored = %q, want (B) prefix", stored.Tasks[0].Raw)
	}
}

func TestDecreasePriority_StepsDown(t *testing.T) {
	uc, repo := newUC(t)
	_, _, _ = uc.AddTask("(A) something")

	_, neu, err := uc.DecreasePriority("0")
	if err != nil {
		t.Fatalf("DecreasePriority: %v", err)
	}
	if neu.Priority != "B" {
		t.Errorf("priority = %q, want B", neu.Priority)
	}
	stored := mustLoad(t, repo, "todo")
	if !strings.HasPrefix(stored.Tasks[0].Raw, "(B) ") {
		t.Errorf("stored = %q, want (B) prefix", stored.Tasks[0].Raw)
	}
}

func TestCurrentContext_ReturnsActiveContextWithTasks(t *testing.T) {
	uc, _ := newUC(t)
	_, _, _ = uc.AddTask("a")
	_, _, _ = uc.AddTask("b")

	ctx, err := uc.CurrentContext()
	if err != nil {
		t.Fatalf("CurrentContext: %v", err)
	}
	if ctx.Name != "todo" {
		t.Errorf("Name = %q, want todo", ctx.Name)
	}
	if len(ctx.Tasks) != 2 {
		t.Errorf("len(Tasks) = %d, want 2", len(ctx.Tasks))
	}
}

func TestCurrentContextName_DefaultsToTodo(t *testing.T) {
	uc, _ := newUC(t)
	name, err := uc.CurrentContextName()
	if err != nil {
		t.Fatalf("CurrentContextName: %v", err)
	}
	if name != "todo" {
		t.Errorf("got %q, want todo", name)
	}
}

func TestSwitchContext_PersistsSanitizedName(t *testing.T) {
	uc, _ := newUC(t)

	got, err := uc.SwitchContext("hello world!")
	if err != nil {
		t.Fatalf("SwitchContext: %v", err)
	}
	// utils.StringToFilename replaces non-word characters with underscores.
	if got != "hello_world_" {
		t.Errorf("sanitized name = %q, want hello_world_", got)
	}
	name, _ := uc.CurrentContextName()
	if name != "hello_world_" {
		t.Errorf("persisted name = %q", name)
	}
}

func TestSwitchContext_EmptyErrors(t *testing.T) {
	uc, _ := newUC(t)
	if _, err := uc.SwitchContext(""); err == nil {
		t.Errorf("expected error on empty name")
	}
}

func TestSearchCurrentContext_FiltersByRegex(t *testing.T) {
	uc, _ := newUC(t)
	_, _, _ = uc.AddTask("buy milk")
	_, _, _ = uc.AddTask("buy bread")
	_, _, _ = uc.AddTask("call alice")

	ctx, matches, err := uc.SearchCurrentContext("^buy")
	if err != nil {
		t.Fatalf("SearchCurrentContext: %v", err)
	}
	if len(ctx.Tasks) != 3 {
		t.Errorf("len(ctx.Tasks) = %d, want 3", len(ctx.Tasks))
	}
	if len(matches) != 2 {
		t.Fatalf("len(matches) = %d, want 2", len(matches))
	}
}

func TestSearchCurrentContext_IsCaseInsensitive(t *testing.T) {
	uc, _ := newUC(t)
	_, _, _ = uc.AddTask("CALL ALICE")
	_, _, _ = uc.AddTask("buy milk")

	_, matches, err := uc.SearchCurrentContext("call")
	if err != nil {
		t.Fatalf("SearchCurrentContext: %v", err)
	}
	if len(matches) != 1 || matches[0].Raw != "CALL ALICE" {
		t.Errorf("matches = %v", matches)
	}
}

func TestSearchCurrentContext_InvalidPatternErrors(t *testing.T) {
	uc, _ := newUC(t)
	if _, _, err := uc.SearchCurrentContext("[unterminated"); err == nil {
		t.Errorf("expected error on invalid regex")
	}
}

func TestListContextNames_ExcludesDone(t *testing.T) {
	uc, _ := newUC(t)
	_, _, _ = uc.AddTaskTo("todo", "a")
	_, _, _ = uc.AddTaskTo("work", "b")
	_, _ = uc.CompleteTask("0") // creates a "done" context behind the scenes

	names, err := uc.ListContextNames()
	if err != nil {
		t.Fatalf("ListContextNames: %v", err)
	}
	got := map[string]bool{}
	for _, n := range names {
		got[n] = true
	}
	if got[DoneContextName] {
		t.Errorf("ListContextNames should exclude %q, got %v", DoneContextName, names)
	}
	if !got["todo"] || !got["work"] {
		t.Errorf("ListContextNames missing expected names; got %v", names)
	}
}
