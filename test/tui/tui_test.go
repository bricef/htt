package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTUI_RendersSeededTasks(t *testing.T) {
	e := newTUIEnv(t)
	e.seedContext("todo", "buy milk", "ship feature")
	e.start("todo")

	v := e.view()
	assertViewContains(t, v, "buy milk")
	assertViewContains(t, v, "ship feature")
}

func TestTUI_RendersEmptyContext(t *testing.T) {
	e := newTUIEnv(t)
	e.seedContext("todo")
	e.start("todo")

	v := e.view()
	// No task rows, but the chrome (header/footer) should still render.
	assertViewContains(t, v, "htt")
	assertViewNotContains(t, v, "buy milk")
}

func TestTUI_NavigateDownMovesCursor(t *testing.T) {
	e := newTUIEnv(t)
	e.seedContext("todo", "first", "second", "third")
	e.start("todo")

	before := e.view()
	beforeFirstIdx := strings.Index(before, "first")
	beforeCursorIdx := strings.Index(before, ">")
	if beforeCursorIdx == -1 || beforeFirstIdx == -1 || beforeCursorIdx > beforeFirstIdx {
		t.Fatalf("expected cursor before 'first' initially; view:\n%s", before)
	}

	e.press('j')
	after := e.view()
	afterFirstIdx := strings.Index(after, "first")
	afterSecondIdx := strings.Index(after, "second")
	afterCursorIdx := strings.Index(after, ">")
	if afterCursorIdx == -1 || afterCursorIdx < afterFirstIdx || afterCursorIdx > afterSecondIdx {
		t.Errorf("expected cursor between 'first' and 'second' after j; view:\n%s", after)
	}
}

func TestTUI_NavigateUpClampsAtTop(t *testing.T) {
	e := newTUIEnv(t)
	e.seedContext("todo", "alpha", "beta")
	e.start("todo")

	e.press('k')
	v := e.view()
	alphaIdx := strings.Index(v, "alpha")
	cursorIdx := strings.Index(v, ">")
	if cursorIdx == -1 || cursorIdx > alphaIdx {
		t.Errorf("cursor should stay at top after k, got:\n%s", v)
	}
}

func TestTUI_AddTask_WritesToContextFile(t *testing.T) {
	e := newTUIEnv(t)
	e.seedContext("todo")
	e.start("todo")

	e.press('n')
	e.type_("buy bread")
	e.pressKey(tea.KeyEnter)

	if got := e.readData("todo.txt"); got != "buy bread\n" {
		t.Errorf("todo.txt = %q, want %q", got, "buy bread\n")
	}

	assertViewContains(t, e.view(), "buy bread")
}

func TestTUI_AddEmptyTask_DoesNotWrite(t *testing.T) {
	e := newTUIEnv(t)
	e.seedContext("todo", "existing")
	e.start("todo")

	e.press('n')
	e.pressKey(tea.KeyEnter)

	if got := e.readData("todo.txt"); got != "existing\n" {
		t.Errorf("todo.txt should be unchanged, got %q", got)
	}
}

func TestTUI_CancelNewTask_DoesNotWrite(t *testing.T) {
	e := newTUIEnv(t)
	e.seedContext("todo", "existing")
	e.start("todo")

	e.press('n')
	e.type_("draft")
	e.pressKey(tea.KeyEsc)

	if got := e.readData("todo.txt"); got != "existing\n" {
		t.Errorf("todo.txt should be unchanged, got %q", got)
	}
	assertViewNotContains(t, e.view(), "draft")
}

func TestTUI_Delete_RemovesTask(t *testing.T) {
	e := newTUIEnv(t)
	e.seedContext("todo", "keep this", "delete this")
	e.start("todo")

	e.press('j')
	e.press('d')

	if got := e.readData("todo.txt"); got != "keep this\n" {
		t.Errorf("todo.txt = %q, want %q", got, "keep this\n")
	}
	assertViewNotContains(t, e.view(), "delete this")
}

func TestTUI_Complete_MovesTaskToDone(t *testing.T) {
	e := newTUIEnv(t)
	e.seedContext("todo", "make tea")
	e.start("todo")

	e.press('x')

	if got := e.readData("todo.txt"); got != "" {
		t.Errorf("todo.txt should be empty after complete, got %q", got)
	}
	done := e.readData("done.txt")
	if !strings.Contains(done, "make tea") {
		t.Errorf("done.txt should contain completed task, got %q", done)
	}
	if !strings.HasPrefix(done, "x ") {
		t.Errorf("done.txt entry should start with 'x ', got %q", done)
	}
}

func TestTUI_IncreasePriority_UpdatesFile(t *testing.T) {
	e := newTUIEnv(t)
	e.seedContext("todo", "(C) something")
	e.start("todo")

	e.press('+')

	if got := e.readData("todo.txt"); !strings.Contains(got, "(B) something") {
		t.Errorf("todo.txt should contain (B) something, got %q", got)
	}
}

func TestTUI_DecreasePriority_UpdatesFile(t *testing.T) {
	e := newTUIEnv(t)
	e.seedContext("todo", "(A) something")
	e.start("todo")

	e.press('-')

	if got := e.readData("todo.txt"); !strings.Contains(got, "(B) something") {
		t.Errorf("todo.txt should contain (B) something, got %q", got)
	}
}

func TestTUI_NextContext_SwitchesCurrent(t *testing.T) {
	e := newTUIEnv(t)
	e.seedContext("todo", "in todo")
	e.seedContext("work", "in work")
	e.seedCurrentContext("todo")
	e.start("todo")

	// Before: cursor on "todo" tab. Press l to advance to next context.
	e.press('l')

	got := e.readData("current-context")
	if got == "todo" {
		t.Errorf("current-context should have changed from 'todo', got %q", got)
	}
}

func TestTUI_Quit_EmitsTeaQuit(t *testing.T) {
	e := newTUIEnv(t)
	e.seedContext("todo", "x")
	e.start("todo")

	_, cmd := e.model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from quit")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}
