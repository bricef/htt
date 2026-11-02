package tui

import (
	"bytes"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
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
	return Model(repo, loaded, nil), repo
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

	m := Model(repo, ctx, nil)
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
	if len(stored.Tasks) != 1 || stored.Tasks[0].Entry() != "buy bread" {
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
	m = Model(repo, todoCtx, nil)

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

// withLogBuffer points the model's logger at a debug-level text
// handler writing to the returned buffer, so tests can read back what
// Act recorded.
func withLogBuffer(m model) (model, *bytes.Buffer) {
	var buf bytes.Buffer
	m.logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return m, &buf
}

func assertLogged(t *testing.T, log string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(log, want) {
			t.Errorf("log missing %q\n--- log ---\n%s", want, log)
		}
	}
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestAct_KeypressLogsActionAtDebug(t *testing.T) {
	m, _ := seedModel(t, "todo", "first", "second")
	m, buf := withLogBuffer(m)

	// Through Update, the way a real keypress arrives.
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}); isQuit(cmd) {
		t.Fatalf("moving down should not quit")
	}
	assertLogged(t, buf.String(), "level=DEBUG", "msg=action", `action="move down"`)
	if strings.Contains(buf.String(), "level=WARN") || strings.Contains(buf.String(), "level=ERROR") {
		t.Errorf("a successful action should log nothing above debug, got:\n%s", buf.String())
	}
}

func TestAct_UnavailableActionLogsWarnAndKeepsBehaviour(t *testing.T) {
	m, _ := seedModel(t, domain.DoneContextName, "x already done")
	m, buf := withLogBuffer(m)

	_, cmd := Do.Act(m)
	if !isQuit(cmd) {
		t.Errorf("do in the done tab should still quit the TUI")
	}
	assertLogged(t, buf.String(), "level=WARN", `msg="action unavailable"`, "action=do", "done tab")

	// Adding a task in the done tab is refused without quitting.
	m.textInput.SetValue("sneaky task")
	_, cmd = AddTask.Act(m)
	if isQuit(cmd) {
		t.Errorf("add task in the done tab should not quit the TUI")
	}
	assertLogged(t, buf.String(), `action="add task"`, "can't go in the done tab")
}

func TestAct_FailedActionLogsErrorAndQuits(t *testing.T) {
	// An empty context has no task 0 to complete.
	m, _ := seedModel(t, "todo")
	m, buf := withLogBuffer(m)

	_, cmd := Do.Act(m)
	if !isQuit(cmd) {
		t.Errorf("a failed do should still quit the TUI")
	}
	assertLogged(t, buf.String(), "level=ERROR", `msg="action failed"`, "action=do", "complete task 0")
}

// seedFileModel is seedModel backed by a FileRepository in a temp dir,
// for the editor actions, which need a real file. Returns the model and
// the context file's path.
func seedFileModel(t *testing.T, contextName string, tasks ...string) (model, string) {
	t.Helper()
	dir := t.TempDir()
	repo := storage.NewFileRepository(dir, dir)
	seed := domain.NewContext(repo, contextName)
	for _, raw := range tasks {
		seed.Tasks = append(seed.Tasks, mustTask(t, raw))
	}
	if err := repo.Save(seed); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := repo.SetCurrent(contextName); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}
	ctx, err := repo.Context(contextName)
	if err != nil {
		t.Fatalf("Context(%q): %v", contextName, err)
	}
	return Model(repo, ctx, nil), repo.ContextPath(contextName)
}

// editorErr runs a real command to get the error type exec produces.
func editorErr(t *testing.T, cmd *exec.Cmd) error {
	t.Helper()
	err := cmd.Run()
	if err == nil {
		t.Fatalf("%v: expected an error", cmd.Args)
	}
	return err
}

func TestEditFile_WithoutEditorStaysOpen(t *testing.T) {
	t.Setenv("EDITOR", "")
	m, _ := seedFileModel(t, "todo", "first")
	m, buf := withLogBuffer(m)

	result, cmd := EditFile.Act(m)
	if cmd != nil {
		t.Errorf("no $EDITOR: expected no command (no quit, no exec), got one")
	}
	if got := asModel(t, result).status; !strings.Contains(got, "$EDITOR") {
		t.Errorf("status = %q, want it to mention $EDITOR", got)
	}
	assertLogged(t, buf.String(), "level=WARN", `action="edit file"`, "$EDITOR is not set")
}

func TestEditFile_WithoutAFileStaysOpen(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	m, _ := seedModel(t, "todo", "first") // memory repo: no file path
	m, buf := withLogBuffer(m)

	result, cmd := EditFile.Act(m)
	if cmd != nil {
		t.Errorf("no file: expected no command, got one")
	}
	if got := asModel(t, result).status; !strings.Contains(got, "no file") {
		t.Errorf("status = %q, want it to say there's no file", got)
	}
	assertLogged(t, buf.String(), "level=WARN", "has no file to edit")
}

func TestEditFile_HandsTheTerminalToTheEditor(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	m, _ := seedFileModel(t, "todo", "first")
	m, buf := withLogBuffer(m)

	result, cmd := EditFile.Act(m)
	if cmd == nil || isQuit(cmd) {
		t.Fatalf("expected an exec command for the editor")
	}
	if got := asModel(t, result).status; got != "" {
		t.Errorf("status = %q, want none before the editor runs", got)
	}
	if strings.Contains(buf.String(), "level=WARN") || strings.Contains(buf.String(), "level=ERROR") {
		t.Errorf("launching the editor should log nothing above debug:\n%s", buf.String())
	}
}

func TestEditorFinished_ReloadsWhatTheEditorSaved(t *testing.T) {
	m, path := seedFileModel(t, "todo", "first", "second", "third")
	m.cursor = 2
	if err := os.WriteFile(path, []byte("only one left\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, cmd := m.Update(editorFinishedMsg{editor: "vi"})
	if isQuit(cmd) {
		t.Fatalf("a clean editor exit should not quit")
	}
	got := asModel(t, result)
	if len(got.context.Tasks) != 1 || got.context.Tasks[0].Entry() != "only one left" {
		t.Errorf("context after editing = %v, want the file's one task", got.context.Tasks)
	}
	if got.cursor != 0 {
		t.Errorf("cursor = %d, want it clamped to the shorter list", got.cursor)
	}
	if got.status != "" {
		t.Errorf("status = %q, want none after a clean exit", got.status)
	}
}

func TestEditorFinished_NonZeroExitReportsAndStillReloads(t *testing.T) {
	m, path := seedFileModel(t, "todo", "first")
	m, buf := withLogBuffer(m)
	if err := os.WriteFile(path, []byte("first\nsaved before failing\n"), 0644); err != nil {
		t.Fatal(err)
	}

	err := editorErr(t, exec.Command("sh", "-c", "exit 3"))
	result, cmd := m.Update(editorFinishedMsg{editor: "vi", err: err})
	if isQuit(cmd) {
		t.Fatalf("an editor error should not quit the TUI")
	}
	got := asModel(t, result)
	if !strings.Contains(got.status, "status 3") {
		t.Errorf("status = %q, want the exit status", got.status)
	}
	if len(got.context.Tasks) != 2 {
		t.Errorf("context has %d tasks, want the 2 on disk", len(got.context.Tasks))
	}
	assertLogged(t, buf.String(), "level=ERROR", `msg="editor exited with an error"`, "editor=vi")
}

func TestEditorFinished_StartFailureReports(t *testing.T) {
	m, _ := seedFileModel(t, "todo", "first")
	m, buf := withLogBuffer(m)

	missing := filepath.Join(t.TempDir(), "no-such-editor")
	err := editorErr(t, exec.Command(missing))
	result, cmd := m.Update(editorFinishedMsg{editor: missing, err: err})
	if isQuit(cmd) {
		t.Fatalf("an editor that won't start should not quit the TUI")
	}
	if got := asModel(t, result).status; !strings.Contains(got, "Couldn't run $EDITOR") {
		t.Errorf("status = %q, want a start failure message", got)
	}
	assertLogged(t, buf.String(), "level=ERROR", `msg="editor failed to run"`)
}

func TestEditorFinished_UnreadableFileStillQuits(t *testing.T) {
	// Staying open on a stale context would let the next keypress
	// save over the user's edits, so a failed reload quits as before.
	if os.Geteuid() == 0 {
		t.Skip("root reads files regardless of mode")
	}
	m, path := seedFileModel(t, "todo", "first")
	m, buf := withLogBuffer(m)
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	_, cmd := m.Update(editorFinishedMsg{editor: "vi"})
	if !isQuit(cmd) {
		t.Errorf("a reload failure should quit")
	}
	assertLogged(t, buf.String(), "level=ERROR", `msg="reload after editing failed"`)
}

func TestStatus_ShownInViewAndClearedByTheNextKey(t *testing.T) {
	m, _ := seedModel(t, "todo", "first", "second")
	m.width, m.height = 80, 24
	m.status = "Set $EDITOR to edit files from here."

	if view := m.View(); !strings.Contains(view, "Set $EDITOR to edit files from here.") {
		t.Errorf("view should show the status line:\n%s", view)
	}
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if got := asModel(t, result).status; got != "" {
		t.Errorf("status = %q after a keypress, want it cleared", got)
	}
}
