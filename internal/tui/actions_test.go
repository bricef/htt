package tui

import (
	"strings"
	"testing"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/storage"
	"github.com/bricef/htt/internal/usecase"
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
func seedModel(t *testing.T, contextName string, tasks ...string) (model, *storage.MemoryRepository) {
	t.Helper()
	repo := storage.NewMemoryRepository()
	uc := usecase.New(repo)

	ctx := &domain.Context{Name: contextName, Tasks: []*domain.Task{}}
	for _, raw := range tasks {
		ctx.Tasks = append(ctx.Tasks, mustTask(t, raw))
	}
	if err := repo.SaveContext(ctx); err != nil {
		t.Fatalf("SaveContext: %v", err)
	}
	if err := repo.SetCurrentContextName(contextName); err != nil {
		t.Fatalf("SetCurrentContextName: %v", err)
	}

	return Model(uc, ctx), repo
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

func TestAction_AddTask_PersistsToRepo(t *testing.T) {
	m, repo := seedModel(t, "todo")
	m.textInput.SetValue("buy bread")

	result, _ := AddTask.Act(m)
	m = asModel(t, result)

	stored, _ := repo.LoadContext("todo")
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

	stored, _ := repo.LoadContext("todo")
	if len(stored.Tasks) != 1 || stored.Tasks[0].Raw != "existing" {
		t.Errorf("repo should be unchanged, got %v", stored.Tasks)
	}
}

func TestAction_Delete_RemovesFromCurrentContext(t *testing.T) {
	m, repo := seedModel(t, "todo", "keep", "delete this")
	m.cursor = 1

	result, _ := Delete.Act(m)
	m = asModel(t, result)

	stored, _ := repo.LoadContext("todo")
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

	src, _ := repo.LoadContext("todo")
	if len(src.Tasks) != 0 {
		t.Errorf("todo should be empty, got %v", src.Tasks)
	}
	done, _ := repo.LoadContext("done")
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

	stored, _ := repo.LoadContext("todo")
	if !strings.Contains(stored.Tasks[0].Raw, "(B)") {
		t.Errorf("expected (B) prefix, got %q", stored.Tasks[0].Raw)
	}
}

func TestAction_DecreasePriority_UpdatesRepo(t *testing.T) {
	m, repo := seedModel(t, "todo", "(A) something")

	result, _ := DecreasePriority.Act(m)
	asModel(t, result)

	stored, _ := repo.LoadContext("todo")
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
	if err := repo.SaveContext(&domain.Context{
		Name:  "work",
		Tasks: []*domain.Task{mustTask(t,"in work")},
	}); err != nil {
		t.Fatal(err)
	}
	// Rebuild model so contexts list includes both. seedModel only knew
	// about "todo" at construction time.
	uc := usecase.New(repo)
	todoCtx, _ := repo.LoadContext("todo")
	m = Model(uc, todoCtx)

	// Find the position of "todo" and step past it.
	for i, c := range m.contexts {
		if c.Name == "todo" {
			m.contextCursor = i
			break
		}
	}

	result, _ := NextContext.Act(m)
	m = asModel(t, result)

	currentName, _ := repo.GetCurrentContextName()
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
