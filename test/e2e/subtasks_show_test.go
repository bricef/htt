package e2e

import (
	"strings"
	"testing"
)

// `htt todo show` indents each subtask under its parent, after its
// number, two spaces a level; --flat, sorted and filtered views list the
// plain lines.

const launchTree = "\n" +
	"  0 (A) Plan the launch +work id:1\n" +
	"  1   Write the announcement p:1 +work\n" +
	"  2   Book the venue p:1 id:2\n" +
	"  3     Call three venues p:2\n" +
	"  4 Pack the van\n" +
	"\n(todo): 5 tasks\n"

func TestSubtasks_ShowIndentsTheTree(t *testing.T) {
	e := newEnv(t)
	e.writeData("todo.txt", launchTodo)

	for _, args := range [][]string{{"todo", "show"}, {"todo"}, {"t", "ls"}} {
		assertEqual(t, "htt "+strings.Join(args, " "), e.mustRun(args...).stdout, launchTree)
	}
	assertContains(t, "todo status", e.mustRun("todo", "status").stdout, launchTree)
	assertContains(t, "status", e.mustRun("status").stdout, launchTree)

	flat := "\n" +
		"  0 (A) Plan the launch +work id:1\n" +
		"  1 Write the announcement p:1 +work\n" +
		"  2 Book the venue p:1 id:2\n" +
		"  3 Call three venues p:2\n" +
		"  4 Pack the van\n" +
		"\n(todo): 5 tasks\n"
	assertEqual(t, "show --flat", e.mustRun("todo", "show", "--flat").stdout, flat)

	assertEqual(t, "show --sort", e.mustRun("todo", "show", "--sort", "-priority").stdout, "\n"+
		"  1 Write the announcement p:1 +work\n"+
		"  2 Book the venue p:1 id:2\n"+
		"  3 Call three venues p:2\n"+
		"  4 Pack the van\n"+
		"  0 (A) Plan the launch +work id:1\n"+
		"\n(todo): 5 of 5 tasks shown.\n")
	assertEqual(t, "show --filter", e.mustRun("todo", "show", "--filter", "has-tag:work").stdout, "\n"+
		"  0 (A) Plan the launch +work id:1\n"+
		"  1 Write the announcement p:1 +work\n"+
		"\n(todo): 2 of 5 tasks shown.\n")
	assertEqual(t, "search", e.mustRun("todo", "search", "venue").stdout, "\n"+
		"  2 Book the venue p:1 id:2\n"+
		"  3 Call three venues p:2\n"+
		"\n(todo): 2 out of 5 tasks matched query.\n")

	// The REPL's view is the tree, and its show takes --flat once.
	r := e.mustRunWithInput("show --flat\nshow\nq\n", "repl")
	assertInOrder(t, "repl", r.stdout, launchTree, flat, launchTree)
	if n := strings.Count(r.stdout, flat); n != 1 {
		t.Errorf("the REPL listed the tasks flat %d times, want once (show --flat):\n%s", n, r.stdout)
	}
}

// A task whose parent was completed shows at the top level, marked; the
// done file and the archive are flat.
func TestSubtasks_ShowMarksADoneParent(t *testing.T) {
	e := newEnv(t)
	e.writeData("todo.txt", launchTodo)
	d := today()
	e.mustRun("todo", "do", "2", "--leave-subtasks") // Book the venue
	r := e.mustRun("todo", "show")
	assertEqual(t, "show", r.stdout, "\n"+
		"  0 (A) Plan the launch +work id:1\n"+
		"  1   Write the announcement p:1 +work\n"+
		"  2 Call three venues p:2  (parent done)\n"+
		"  3 Pack the van\n"+
		"\n(todo): 4 tasks\n")
	assertEqual(t, "show: stderr", r.stderr, "")
	assertEqual(t, "show --flat", e.mustRun("todo", "show", "--flat").stdout, "\n"+
		"  0 (A) Plan the launch +work id:1\n"+
		"  1 Write the announcement p:1 +work\n"+
		"  2 Call three venues p:2\n"+
		"  3 Pack the van\n"+
		"\n(todo): 4 tasks\n")

	e.mustRun("todo", "do", "0", "--with-subtasks")
	e.mustRun("todo", "context", "done")
	assertEqual(t, "done", e.mustRun("todo", "show").stdout, "\n"+
		"  0 x "+d+" Book the venue p:1 id:2 context:todo\n"+
		"  1 x (A) "+d+" Plan the launch +work id:1 context:todo\n"+
		"  2 x "+d+" Write the announcement p:1 +work context:todo\n"+
		"\n(done): 3 tasks\n")
}
