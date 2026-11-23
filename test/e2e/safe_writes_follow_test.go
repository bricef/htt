//go:build !windows

package e2e

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Commands that wait for you (docs/design/safe-writes.md, "Limits"):
// `htt todo edit N` while the editor is open, and `htt todo do N` while
// its question about open subtasks waits. Another htt can add or remove
// tasks above task N meanwhile; the command follows the task by its
// text, and leaves it alone if it changed. Unix only: the editor is a
// shell script, and the question needs a pseudo-terminal.

// editorDoing writes an editor script that, while it's open, runs htt
// with args, as another terminal would, then saves text as the edit.
func editorDoing(t *testing.T, e *env, text string, args ...string) string {
	t.Helper()
	script := filepath.Join(e.home, "editor.sh")
	body := "#!/bin/sh\n" +
		"'" + binaryPath + "' --no-color " + strings.Join(args, " ") + " > /dev/null || exit 1\n" +
		"echo '" + text + "' > \"$1\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

const followTodo = "alpha\nbravo\ncharlie\ndelta pay the rent due:2026-12-01\necho\n"

// The review's case: `htt todo edit 2` opens charlie; while the editor
// is open, another htt completes task 0. The edit replaces charlie, not
// delta, which was then task 2, and says so.
func TestSafeWrites_AnEditFollowsItsTask(t *testing.T) {
	e := safeEnv(t)
	e.writeData("todo.txt", followTodo)
	e.extraEnv = []string{"EDITOR=" + editorDoing(t, e, "charlie EDITED", "todo", "do", "0"), "VISUAL=", "TMPDIR=" + t.TempDir()}

	r := e.mustRun("todo", "edit", "2")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "bravo\ncharlie EDITED\ndelta pay the rent due:2026-12-01\necho\n")
	assertContains(t, "done.txt", e.readData("done.txt"), " alpha context:todo")
	assertContains(t, "stdout", r.stdout, "Before: charlie\nAfter:  charlie EDITED\n")
}

// A task changed meanwhile is left alone: nothing is saved, htt says
// so, and the edit is kept in the file it names.
func TestSafeWrites_AnEditOfATaskChangedMeanwhileIsKept(t *testing.T) {
	e := safeEnv(t)
	e.writeData("todo.txt", followTodo)
	e.extraEnv = []string{"EDITOR=" + editorDoing(t, e, "charlie EDITED", "todo", "replace", "2", "charlie changed elsewhere"), "VISUAL=", "TMPDIR=" + t.TempDir()}

	r := e.run("todo", "edit", "2")
	if r.exitCode == 0 {
		t.Fatalf("htt todo edit saved over a task changed meanwhile:\n%s", r.stdout)
	}
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "alpha\nbravo\ncharlie changed elsewhere\ndelta pay the rent due:2026-12-01\necho\n")
	m := regexp.MustCompile(`task 2 changed or went while you were editing it, so nothing was saved; your edit is kept in (\S+)`).FindStringSubmatch(r.stderr)
	if m == nil {
		t.Fatalf("stderr doesn't say so: %q", r.stderr)
	}
	kept, err := os.ReadFile(m[1])
	if err != nil {
		t.Fatalf("the edit isn't where htt says: %v", err)
	}
	assertEqual(t, "the edit kept", string(kept), "charlie EDITED\n")
	if strings.Contains(r.stdout, "After:") {
		t.Errorf("stdout claims a change: %q", r.stdout)
	}
}

const followSubtasks = "alpha\nbravo id:1\nbravo-sub p:1\ncharlie id:2\ncharlie-sub-one p:2\ncharlie-sub-two p:2\ndelta\n"

// The question's answer follows the task asked about: `htt todo do 1`
// asks about bravo; while it waits, another htt completes task 0; A
// completes bravo and its subtask, not charlie, then task 1.
func TestSafeWrites_TheAnswerFollowsItsTask(t *testing.T) {
	e := safeEnv(t)
	e.writeData("todo.txt", followSubtasks)
	term := e.startInPty("todo", "do", "1")
	term.waitFor("Anything else cancels.\r\n")
	e.mustRun("todo", "do", "0")
	term.send("A\r")
	term.waitFor("Completed: x " + today() + " bravo-sub p:1 context:todo\r\n")
	term.exit()
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "charlie id:2\ncharlie-sub-one p:2\ncharlie-sub-two p:2\ndelta\n")
}

// The answer is about the task and the subtasks the question counted:
// when the task changed, or its subtasks did, while it waited, nothing
// is completed, and htt says why.
func TestSafeWrites_TheAnswerToAQuestionOverTakenRefuses(t *testing.T) {
	for _, tc := range []struct {
		name, says string
		elsewhere  []string
	}{
		{"the task changed", "task 1 changed or went while you were answering, so nothing was completed", []string{"todo", "priority", "1", "A"}},
		{"a subtask added", "task 1's open subtasks changed while you were answering, so nothing was completed", []string{"todo", "add", "--under", "1", "another"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := safeEnv(t)
			e.writeData("todo.txt", followSubtasks)
			term := e.startInPty("todo", "do", "1")
			term.waitFor("Anything else cancels.\r\n")
			e.mustRun(tc.elsewhere...)
			before := e.readData("todo.txt")
			term.send("A\r")
			term.waitFor(tc.says)
			select {
			case <-term.exited:
				if term.waitErr == nil {
					t.Error("htt exited 0")
				}
			case <-time.After(10 * time.Second):
				t.Fatalf("htt didn't exit; it wrote:\n%q", term.output())
			}
			assertEqual(t, "todo.txt", e.readData("todo.txt"), before)
			if e.dataExists("done.txt") {
				t.Errorf("done.txt: %q", e.readData("done.txt"))
			}
		})
	}
}
