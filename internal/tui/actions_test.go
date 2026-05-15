package tui

import (
	"strings"
	"testing"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/storage"
	tea "github.com/charmbracelet/bubbletea"
)

func mustTask(t *testing.T, raw string) *domain.Task {
	t.Helper()
	task, err := domain.NewTask(raw)
	if err != nil {
		t.Fatalf("domain.NewTask(%q): %v", raw, err)
	}
	return task
}

// seedModel builds a minimal model rooted at the named context with the
// given task lines, backed by an in-memory repository. Returns the model
// and the repo so tests can inspect post-action state.
//
// The context is round-tripped through repo.Context so it carries a
// wired repo — Step 3 persistent methods (Delete, Complete, …) need it.
func seedModel(t *testing.T, contextName string, tasks ...string) (model, *storage.MemoryRepository) {
	t.Helper()
	repo := storage.NewMemoryRepository()

	seed := &domain.Context{Name: contextName, Tasks: []*domain.Task{}}
	for _, raw := range tasks {
		seed.Tasks = append(seed.Tasks, mustTask(t, raw))
	}
	if err := repo.Save(seed); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := repo.SetCurrent(contextName); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}

	loaded, err := repo.Context(contextName)
	if err != nil {
		t.Fatalf("Context(%q): %v", contextName, err)
	}
	return Model(repo, loaded), repo
}

// asModel asserts the tea.Model returned by an action is back to model.
func asModel(t *testing.T, m tea.Model) model {
	t.Helper()
	mm, ok := m.(model)
	if !ok {
		t.Fatalf("action did not return model, got %T", m)
	}
	return mm
}

func TestModel_FreshInstall_ContextCursorIsNonNegative(t *testing.T) {
	// bug_006: On a fresh install no context files exist yet, so the
	// synthetic done tab is the only entry in m.contexts. The default
	// loaded ctx is "todo" — slices.IndexFunc returns -1 (no match)
	// and m.contextCursor used to land at -1. Pressing 'h' (PreviousContext)
	// would then panic at m.contexts[-1].Name. The fix clamps to 0.
	repo := storage.NewMemoryRepository()
	ctx, err := repo.CurrentContext()
	if err != nil {
		t.Fatalf("CurrentContext: %v", err)
	}

	m := Model(repo, ctx)
	if m.contextCursor < 0 {
		t.Errorf("contextCursor = %d on fresh install, want >= 0", m.contextCursor)
	}

	// PreviousContext must not panic now; defending against future
	// regressions where the clamp gets removed.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PreviousContext panicked on fresh install: %v", r)
		}
	}()
	_, _ = PreviousContext.Act(m)
}

func TestAction_AddTask_PersistsToRepo(t *testing.T) {
	m, repo := seedModel(t, "todo")
	m.textInput.SetValue("buy bread")

	result, _ := AddTask.Act(m)
	m = asModel(t, result)

	stored, _ := repo.Context("todo")
	if len(stored.Tasks) != 1 || stored.Tasks[0].Raw != "buy bread" {
		t.Errorf("repo state = %v", stored.Tasks)
	}
	if len(m.context.Tasks) != 1 {
		t.Errorf("model context not refreshed; len = %d", len(m.context.Tasks))
	}
}

func TestAction_AddTask_EmptyDoesNotPersist(t *testing.T) {
	m, repo := seedModel(t, "todo", "existing")
	m.textInput.SetValue("")

	AddTask.Act(m)

	stored, _ := repo.Context("todo")
	if len(stored.Tasks) != 1 || stored.Tasks[0].Raw != "existing" {
		t.Errorf("repo should be unchanged, got %v", stored.Tasks)
	}
}

func TestAction_AddTask_InDoneViewDoesNotPersist(t *testing.T) {
	// bug_008: AddTask in the done view used to write an uncompleted
	// entry (no `x ` prefix) into done.txt, producing a phantom task
	// that rendered as completed but parsed as not completed.
	m, repo := seedModel(t, domain.DoneContextName, "x already done")
	m.textInput.SetValue("phantom entry")

	AddTask.Act(m)

	stored, _ := repo.Context(domain.DoneContextName)
	if len(stored.Tasks) != 1 || stored.Tasks[0].Raw != "x already done" {
		t.Errorf("done context should be unchanged after AddTask, got %v", stored.Tasks)
	}
}

func TestAction_Delete_RemovesFromCurrentContext(t *testing.T) {
	m, repo := seedModel(t, "todo", "keep", "delete this")
	m.cursor = 1

	result, _ := Delete.Act(m)
	m = asModel(t, result)

	stored, _ := repo.Context("todo")
	if len(stored.Tasks) != 1 || stored.Tasks[0].Raw != "keep" {
		t.Errorf("repo state = %v", stored.Tasks)
	}
	if len(m.context.Tasks) != 1 {
		t.Errorf("model not refreshed; len = %d", len(m.context.Tasks))
	}
}

func TestAction_Do_MovesTaskToDone(t *testing.T) {
	m, repo := seedModel(t, "todo", "make tea")

	result, _ := Do.Act(m)
	m = asModel(t, result)

	src, _ := repo.Context("todo")
	if len(src.Tasks) != 0 {
		t.Errorf("todo should be empty, got %v", src.Tasks)
	}
	done, _ := repo.Context("done")
	if len(done.Tasks) != 1 || !strings.Contains(done.Tasks[0].Raw, "make tea") {
		t.Errorf("done should have completed task, got %v", done.Tasks)
	}
	if len(m.context.Tasks) != 0 {
		t.Errorf("model not refreshed after Do; len = %d", len(m.context.Tasks))
	}
}

func TestAction_Do_InDoneContext_Quits(t *testing.T) {
	m, _ := seedModel(t, "done", "x already done")

	_, cmd := Do.Act(m)
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd, got nil")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestAction_IncreasePriority_UpdatesRepo(t *testing.T) {
	m, repo := seedModel(t, "todo", "(C) something")

	result, _ := IncreasePriority.Act(m)
	asModel(t, result)

	stored, _ := repo.Context("todo")
	if !strings.Contains(stored.Tasks[0].Raw, "(B)") {
		t.Errorf("expected (B) prefix, got %q", stored.Tasks[0].Raw)
	}
}

func TestAction_DecreasePriority_UpdatesRepo(t *testing.T) {
	m, repo := seedModel(t, "todo", "(A) something")

	result, _ := DecreasePriority.Act(m)
	asModel(t, result)

	stored, _ := repo.Context("todo")
	if !strings.Contains(stored.Tasks[0].Raw, "(B)") {
		t.Errorf("expected (B) prefix, got %q", stored.Tasks[0].Raw)
	}
}

func TestAction_Down_MovesCursorWithinBounds(t *testing.T) {
	m, _ := seedModel(t, "todo", "a", "b")
	result, _ := Down.Act(m)
	m = asModel(t, result)
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}

	result, _ = Down.Act(m)
	m = asModel(t, result)
	if m.cursor != 1 {
		t.Errorf("cursor should clamp at last index; got %d", m.cursor)
	}
}

func TestAction_Delete_ClampsCursorAfterLastTaskGoes(t *testing.T) {
	// bug_013: With cursor on the last task, Delete leaves m.cursor
	// pointing one past the new end of m.context.Tasks. The next
	// mutating keypress then hits an out-of-range error and quits the
	// TUI. clampCursor runs after refresh so the cursor stays valid.
	m, _ := seedModel(t, "todo", "first", "second")
	m.cursor = 1

	result, _ := Delete.Act(m)
	m = asModel(t, result)

	if len(m.context.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d after delete, want 1", len(m.context.Tasks))
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d after deleting last task, want 0 (clamped)", m.cursor)
	}
}

func TestAction_Do_ClampsCursorAfterLastTaskGoes(t *testing.T) {
	// Same bug shape via Do.
	m, _ := seedModel(t, "todo", "first", "second")
	m.cursor = 1

	result, _ := Do.Act(m)
	m = asModel(t, result)

	if len(m.context.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d after do, want 1", len(m.context.Tasks))
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d after completing last task, want 0 (clamped)", m.cursor)
	}
}

func TestAction_Up_ClampsAtZero(t *testing.T) {
	m, _ := seedModel(t, "todo", "a")
	result, _ := Up.Act(m)
	m = asModel(t, result)
	if m.cursor != 0 {
		t.Errorf("cursor should stay at 0, got %d", m.cursor)
	}
}

func TestAction_NextContext_SwitchesAndRefreshes(t *testing.T) {
	m, repo := seedModel(t, "todo", "in todo")
	if err := repo.Save(&domain.Context{
		Name:  "work",
		Tasks: []*domain.Task{mustTask(t, "in work")},
	}); err != nil {
		t.Fatal(err)
	}
	// Rebuild model so contexts list includes both. seedModel only knew
	// about "todo" at construction time.
	todoCtx, _ := repo.Context("todo")
	m = Model(repo, todoCtx)

	// Find the position of "todo" and step past it.
	for i, c := range m.contexts {
		if c.Name == "todo" {
			m.contextCursor = i
			break
		}
	}

	result, _ := NextContext.Act(m)
	m = asModel(t, result)

	currentName, _ := repo.CurrentContextName()
	if currentName == "todo" {
		t.Errorf("current context should have changed from todo, got %q", currentName)
	}
}

func TestAction_Help_TogglesShowHelp(t *testing.T) {
	m, _ := seedModel(t, "todo")
	if m.showHelp {
		t.Fatal("expected showHelp false initially")
	}
	result, _ := Help.Act(m)
	m = asModel(t, result)
	if !m.showHelp {
		t.Errorf("showHelp should be true after toggle")
	}
}

func TestAction_Quit_EmitsTeaQuit(t *testing.T) {
	m, _ := seedModel(t, "todo")
	_, cmd := Quit.Act(m)
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", msg)
	}
}
