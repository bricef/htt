package e2e

import (
	"strings"
	"testing"
)

// Completing a task with open subtasks asks first; without a terminal
// to ask on, the flags answer, and without them nothing is completed.

func TestSubtasks_DoWithoutATerminalNeedsAFlag(t *testing.T) {
	e := newEnv(t)
	e.writeData("todo.txt", launchTodo)
	d := today()

	// No terminal: stdin unconnected, then piped (an answer on a pipe
	// isn't taken as one).
	for _, input := range []string{"", "x\n", "A\n"} {
		r := e.runWithInput(input, "todo", "do", "0")
		if r.exitCode == 0 {
			t.Errorf("input %q: exit 0", input)
		}
		assertEqual(t, "refusal", r.stderr, "❌ task 0 has 3 open subtasks, and there's no terminal to ask on: "+
			"give --leave-subtasks to complete it alone, or --with-subtasks to complete them too\n")
		assertEqual(t, "stdout", r.stdout, "")
		assertEqual(t, "todo.txt", e.readData("todo.txt"), launchTodo)
	}

	r := e.run("todo", "do", "0", "--leave-subtasks", "--with-subtasks")
	if r.exitCode == 0 || !strings.Contains(r.stderr, "--leave-subtasks and --with-subtasks exclude each other") {
		t.Errorf("both flags: exit %d, stderr %q", r.exitCode, r.stderr)
	}

	// --leave-subtasks: the task alone; its subtasks stay, naming it,
	// and show at the top level without a warning.
	r = e.mustRun("todo", "do", "2", "--leave-subtasks") // Book the venue
	assertEqual(t, "--leave-subtasks: stdout", r.stdout, "Completed: x "+d+" Book the venue p:1 id:2 context:todo\n")
	assertEqual(t, "todo.txt", e.readData("todo.txt"),
		"(A) Plan the launch +work id:1\nWrite the announcement p:1 +work\nCall three venues p:2\nPack the van\n")
	assertEqual(t, "show: stderr", e.mustRun("todo", "show").stderr, "")

	// --with-subtasks: every open task below, at any depth.
	e.writeData("todo.txt", launchTodo)
	e.writeData("done.txt", "")
	r = e.mustRun("todo", "do", "0", "--with-subtasks")
	assertEqual(t, "--with-subtasks: stdout", r.stdout,
		"Completed: x (A) "+d+" Plan the launch +work id:1 context:todo\n"+
			"Completed: x "+d+" Write the announcement p:1 +work context:todo\n"+
			"Completed: x "+d+" Book the venue p:1 id:2 context:todo\n"+
			"Completed: x "+d+" Call three venues p:2 context:todo\n")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "Pack the van\n")

	// A task without open subtasks completes as before, flags or not.
	r = e.mustRun("todo", "do", "0", "--with-subtasks")
	assertEqual(t, "no subtasks: stdout", r.stdout, "Completed: x "+d+" Pack the van context:todo\n")
}

// The REPL reading a pipe has no terminal either: the command fails, and
// the session goes on.
func TestRepl_DoWithoutATerminalNeedsAFlag(t *testing.T) {
	e := newEnv(t)
	e.writeData("todo.txt", launchTodo)
	r := e.mustRunWithInput("do 0\nx\ndo 0 --leave-subtasks\nq\n", "repl")
	assertContains(t, "stderr", r.stderr, "task 0 has 3 open subtasks, and there's no terminal to ask on")
	assertContains(t, "stdout", r.stdout, "Completed: x (A) "+today()+" Plan the launch +work id:1 context:todo")
	// x on its own line was a command, `todo x`, which wants a task.
	assertContains(t, "stderr", r.stderr, "accepts 1 arg(s), received 0")
	// The flag doesn't stick to the next command: Book the venue, now
	// task 1, still asks.
	r = e.mustRunWithInput("do 0 --with-subtasks\ndo 0\nq\n", "repl")
	assertContains(t, "stderr", r.stderr, "task 0 has 1 open subtask, and there's no terminal to ask on")
}
