//go:build !windows

package e2e

import (
	"bytes"
	"os/exec"
	"syscall"
	"testing"

	"github.com/creack/pty"
)

// The question before completing a task with open subtasks, asked on a
// terminal: a pseudo-terminal here, as in a terminal window.

const subtasksQuestion = "Task 0 has 3 open subtasks.\r\n" +
	"[x] Mark it done, and leave the subtasks as they are.\r\n" +
	"[A] Mark the subtasks done too.\r\n" +
	"Anything else cancels.\r\n"

func TestSubtasks_DoAsksOnATerminal(t *testing.T) {
	d := today()
	afterX := "Write the announcement p:1 +work\nBook the venue p:1 id:2\nCall three venues p:2\nPack the van\n"
	cases := []struct {
		name, keys, says, todo string
	}{
		{"x: the task alone", "x\r", "Completed: x (A) " + d + " Plan the launch +work id:1 context:todo\r\n", afterX},
		{"A: every open task below", "A\r", "Completed: x (A) " + d + " Plan the launch +work id:1 context:todo\r\n" +
			"Completed: x " + d + " Write the announcement p:1 +work context:todo\r\n" +
			"Completed: x " + d + " Book the venue p:1 id:2 context:todo\r\n" +
			"Completed: x " + d + " Call three venues p:2 context:todo\r\n", "Pack the van\n"},
		{"a lower-case a cancels: answers are exact", "a\r", "Cancelled: nothing changed.\r\n", launchTodo},
		{"X cancels", "X\r", "Cancelled: nothing changed.\r\n", launchTodo},
		{"x with a space cancels", " x\r", "Cancelled: nothing changed.\r\n", launchTodo},
		{"anything else cancels", "yes\r", "Cancelled: nothing changed.\r\n", launchTodo},
		{"Enter alone cancels", "\r", "Cancelled: nothing changed.\r\n", launchTodo},
		{"the end of the input cancels", "\x04", "Cancelled: nothing changed.\r\n", launchTodo},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.writeConfig("check_for_updates: false\n")
			e.writeData("todo.txt", launchTodo)
			term := e.startInPty("todo", "do", "0")
			term.waitFor(subtasksQuestion)
			term.send(tc.keys)
			term.waitFor(tc.says)
			term.exit()
			assertEqual(t, "todo.txt", e.readData("todo.txt"), tc.todo)
		})
	}
}

// A terminal for input but not for output (the output piped or
// redirected) is no terminal to ask on.
func TestSubtasks_DoNeedsStdoutOnATerminalToo(t *testing.T) {
	e := newEnv(t)
	e.writeConfig("check_for_updates: false\n")
	e.writeData("todo.txt", launchTodo)
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ptmx.Close(); _ = tty.Close() }()
	cmd := exec.Command(binaryPath, "--no-color", "todo", "do", "0")
	cmd.Env = append(e.homeEnv(), e.extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, &stdout, &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	err = cmd.Run()
	if _, failed := err.(*exec.ExitError); !failed {
		t.Errorf("exit: %v, want a failure", err)
	}
	assertContains(t, "stderr", stderr.String(), "there's no terminal to ask on: give --leave-subtasks")
	assertEqual(t, "stdout", stdout.String(), "")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), launchTodo)
}

// In the REPL the question takes the terminal from the line reader, as
// the TUI does, so the answer is the question's and not a command; the
// REPL then reads every key typed after, the first included.
func TestRepl_DoAsksOnTheTerminal(t *testing.T) {
	e := newEnv(t)
	e.writeConfig("check_for_updates: false\n")
	e.writeData("todo.txt", launchTodo)
	term := e.startInPty("repl")
	term.waitFor("htt(todo)> ")

	term.send("do 2\r") // Book the venue
	term.waitFor("Task 2 has 1 open subtask.\r\n")
	term.waitFor("Anything else cancels.\r\n")
	term.send("nope\r")
	term.waitFor("Cancelled: nothing changed.")
	term.waitFor("htt(todo)> ")

	term.send("do 0\r")
	term.waitFor(subtasksQuestion)
	term.send("A\r")
	term.waitFor("Completed: x " + today() + " Call three venues p:2 context:todo")
	term.waitFor("htt(todo)> ")

	term.send("add after the question\r")
	term.waitFor("Added: " + today() + " after the question")
	term.waitFor("htt(todo)> ")
	term.send("q\r")
	term.exit()
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "Pack the van\n"+today()+" after the question\n")
}
