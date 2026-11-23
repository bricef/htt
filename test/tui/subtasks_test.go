package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Subtasks in the TUI against files, as `htt interactive` runs it: the
// tree, the question before completing, Tab, Shift-Tab and s, each
// written to the file, and nothing written to the terminal.
func TestTUI_Subtasks(t *testing.T) {
	e := newTUIEnv(t)
	e.seedContext("todo",
		"(A) Plan the launch +work id:1",
		"Write the announcement p:1 +work",
		"Book the venue p:1 id:2",
		"Call three venues p:2",
		"Pack the van",
	)
	e.seedCurrentContext("todo")
	d := time.Now().Format("2006-01-02")

	written := terminalOutput(t, func() {
		e.start("todo")
		v := e.view()
		assertViewContains(t, v, "   1   Write the announcement p:1 +work")
		assertViewContains(t, v, "   3     Call three venues p:2")

		// Pack the van under Plan, then back out.
		for range 4 {
			e.pressKey(tea.KeyDown)
		}
		e.pressKey(tea.KeyTab)
		assertViewContains(t, e.view(), ">    4   Pack the van p:1")
		e.pressKey(tea.KeyShiftTab)
		assertViewContains(t, e.view(), ">    4 Pack the van")

		// A subtask of Book the venue.
		e.pressKey(tea.KeyUp)
		e.pressKey(tea.KeyUp)
		e.press('s')
		e.type_("Ring Ann")
		e.pressKey(tea.KeyEnter)
		assertViewContains(t, e.view(), ">    4     "+d+" Ring Ann p:2")

		// Book the venue, alone: its subtasks stay, under a done parent.
		e.pressKey(tea.KeyUp)
		e.pressKey(tea.KeyUp)
		e.press('x')
		assertViewContains(t, e.view(), "Task 2 has 2 open subtasks.")
		e.press('x')
		v = e.view()
		assertViewContains(t, v, "Call three venues p:2  (parent done)")
		assertViewContains(t, v, d+" Ring Ann p:2  (parent done)")
	})
	if written != "" {
		t.Errorf("the TUI wrote to the terminal:\n%s", written)
	}
	// Book's subtasks stay where they were, now at the top level.
	assertEqualFile(t, e, "todo.txt",
		"(A) Plan the launch +work id:1\n"+
			"Write the announcement p:1 +work\n"+
			"Call three venues p:2\n"+
			d+" Ring Ann p:2\n"+
			"Pack the van\n")
	assertEqualFile(t, e, "done.txt", "x "+d+" Book the venue p:1 id:2 context:todo\n")
}

func assertEqualFile(t *testing.T, e *tuiEnv, name, want string) {
	t.Helper()
	if got := e.readData(name); got != want {
		t.Errorf("%s:\n%s\nwant:\n%s", name, got, want)
	}
}
