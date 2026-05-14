package e2e

import (
	"strings"
	"testing"
)

func TestTodoAdd_WritesTaskToCurrentContextFile(t *testing.T) {
	e := newEnv(t)

	r := e.mustRun("todo", "add", "buy milk")
	assertContains(t, "stdout", r.stdout, "Added:")
	assertContains(t, "stdout", r.stdout, "buy milk")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "buy milk\n")
}

func TestTodoAdd_AppendsMultipleTasks(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "first task")
	e.mustRun("todo", "add", "second task")
	e.mustRun("todo", "add", "third task")

	assertEqual(t, "todo.txt", e.readData("todo.txt"),
		"first task\nsecond task\nthird task\n")
}

func TestTodoShow_EmptyContext(t *testing.T) {
	e := newEnv(t)

	r := e.mustRun("todo", "show")
	assertContains(t, "stdout", r.stdout, "Context is empty")
}

func TestTodoShow_ListsTasks(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "alpha")
	e.mustRun("todo", "add", "beta")

	r := e.mustRun("todo", "show")
	assertContains(t, "stdout", r.stdout, "alpha")
	assertContains(t, "stdout", r.stdout, "beta")
	assertContains(t, "stdout", r.stdout, "2 tasks")
}

func TestTodoAddTo_WritesToNamedContext(t *testing.T) {
	e := newEnv(t)

	r := e.mustRun("todo", "add-to", "work", "ship feature")
	assertContains(t, "stdout", r.stdout, "Added:")
	assertContains(t, "stdout", r.stdout, "ship feature")
	assertContains(t, "stdout", r.stdout, "work")
	assertEqual(t, "work.txt", e.readData("work.txt"), "ship feature\n")
	if e.dataExists("todo.txt") {
		got := e.readData("todo.txt")
		if got != "" {
			t.Errorf("todo.txt should be empty or absent, got %q", got)
		}
	}
}

func TestTodoDo_MovesTaskToDone(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "make tea")
	r := e.mustRun("todo", "do", "0")

	assertContains(t, "stdout", r.stdout, "Completed:")
	assertContains(t, "stdout", r.stdout, "make tea")

	assertEqual(t, "todo.txt", e.readData("todo.txt"), "")
	done := e.readData("done.txt")
	if !strings.Contains(done, "make tea") {
		t.Errorf("done.txt should contain completed task, got %q", done)
	}
	if !strings.HasPrefix(done, "x ") {
		t.Errorf("done.txt entry should start with 'x ', got %q", done)
	}
}

func TestTodoDelete_RemovesTask(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "keep this")
	e.mustRun("todo", "add", "delete this")
	r := e.mustRun("todo", "delete", "1")

	assertContains(t, "stdout", r.stdout, "Deleted task:")
	assertContains(t, "stdout", r.stdout, "delete this")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "keep this\n")
}

func TestTodoMove_BetweenContexts(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "moveme")
	r := e.mustRun("todo", "move", "0", "work")

	assertContains(t, "stdout", r.stdout, "Moved")
	assertContains(t, "stdout", r.stdout, "moveme")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "")
	assertEqual(t, "work.txt", e.readData("work.txt"), "moveme\n")
}

func TestTodoPriority_SetsExplicitPriority(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "urgent thing")
	r := e.mustRun("todo", "priority", "0", "A")

	assertContains(t, "stdout", r.stdout, "After:")
	assertContains(t, "stdout", r.stdout, "(A)")
	assertContains(t, "stdout", r.stdout, "urgent thing")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "(A) urgent thing\n")
}

func TestTodoPriorityIncrease_LowersLetter(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "(C) something")
	e.mustRun("todo", "+", "0")

	assertContains(t, "todo.txt", e.readData("todo.txt"), "(B) something")
}

// NOTE: `htt todo - <id>` is not reachable via subprocess args because
// Cobra resolves "-" as a flag prefix before any "--" separator. This is a
// pre-existing CLI bug; the `+` test above covers the symmetric priority-
// change code path. Revisit when the CLI layer is refactored.

func TestTodoReplace_OverwritesTask(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "old text")
	r := e.mustRun("todo", "replace", "0", "new", "text")

	assertContains(t, "stdout", r.stdout, "After:")
	assertContains(t, "stdout", r.stdout, "new text")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "new text\n")
}

func TestTodoSearch_MatchesByRegex(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "buy bread")
	e.mustRun("todo", "add", "buy milk")
	e.mustRun("todo", "add", "call alice")

	r := e.mustRun("todo", "search", "^buy")
	assertContains(t, "stdout", r.stdout, "buy bread")
	assertContains(t, "stdout", r.stdout, "buy milk")
	if strings.Contains(r.stdout, "call alice") {
		t.Errorf("search ^buy should not include 'call alice', got:\n%s", r.stdout)
	}
	assertContains(t, "stdout", r.stdout, "2 out of 3")
}

func TestTodoContext_ShowsCurrent(t *testing.T) {
	e := newEnv(t)

	r := e.mustRun("todo", "context")
	assertContains(t, "stdout", r.stdout, "todo")
}

func TestTodoContext_SwitchesAndPersists(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "seed")

	r := e.mustRun("todo", "context", "work")
	assertContains(t, "stdout", r.stdout, "Now using context: work")

	e.mustRun("todo", "add", "task in work")
	assertEqual(t, "work.txt", e.readData("work.txt"), "task in work\n")

	r2 := e.mustRun("todo", "context")
	assertContains(t, "stdout", r2.stdout, "work")
}

func TestTodoStatus_ListsContextsAndCurrent(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "first")
	e.mustRun("todo", "add-to", "work", "ship feature")

	r := e.mustRun("todo", "status")
	assertContains(t, "stdout", r.stdout, "Available Contexts")
	assertContains(t, "stdout", r.stdout, "todo")
	assertContains(t, "stdout", r.stdout, "work")
	assertContains(t, "stdout", r.stdout, "Current Context")
	assertContains(t, "stdout", r.stdout, "first")
}

func TestTodoDelete_OutOfRangeFails(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "only task")
	r := e.run("todo", "delete", "99")

	if r.exitCode == 0 {
		t.Errorf("expected non-zero exit, got 0\nstdout:\n%s\nstderr:\n%s", r.stdout, r.stderr)
	}
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "only task\n")
}

func TestTodoPriority_InvalidPriorityFails(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "thing")
	r := e.run("todo", "priority", "0", "Z")

	if r.exitCode == 0 {
		t.Errorf("expected non-zero exit, got 0\nstdout:\n%s\nstderr:\n%s", r.stdout, r.stderr)
	}
}
