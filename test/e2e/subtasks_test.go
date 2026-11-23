package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// Subtasks (docs/design/subtasks.md): a parent carries id:N, each of its
// subtasks p:N, and htt keeps every subtree below its parent in the file.

// launchTodo is the design's example, as a file htt wrote.
const launchTodo = "(A) Plan the launch +work id:1\n" +
	"Write the announcement p:1 +work\n" +
	"Book the venue p:1 id:2\n" +
	"Call three venues p:2\n" +
	"Pack the van\n"

func TestSubtasks_AddUnder(t *testing.T) {
	e := newEnv(t)
	d := today()
	e.mustRun("todo", "add", "Plan the launch")
	e.mustRun("todo", "add", "Pack the van")

	r := e.mustRun("todo", "add", "--under", "0", "Write the announcement")
	assertEqual(t, "add --under: stdout", r.stdout,
		"Added: "+d+" Write the announcement p:1\nUnder: "+d+" Plan the launch id:1\n")
	r = e.mustRun("todo", "add", "--under", "1", "Find a photo")
	assertEqual(t, "add --under a subtask: stdout", r.stdout,
		"Added: "+d+" Find a photo p:2\nUnder: "+d+" Write the announcement p:1 id:2\n")
	e.mustRun("todo", "add", "--under", "0", "--due", "2026-12-01", "Book the venue")
	e.mustRun("todo", "add", "--under", "0", "(A) Set the date")
	assertEqual(t, "todo.txt", e.readData("todo.txt"),
		d+" Plan the launch id:1\n"+
			"(A) "+d+" Set the date p:1\n"+
			d+" Write the announcement p:1 id:2\n"+
			d+" Find a photo p:2\n"+
			d+" Book the venue due:2026-12-01 p:1\n"+
			d+" Pack the van\n")

	// add-to takes --under, naming a task of that context.
	e.writeData("work.txt", "Review id:4\nother p:9\n")
	r = e.mustRun("todo", "add-to", "work", "--under", "0", "read the PR")
	assertEqual(t, "add-to --under: stdout", r.stdout,
		"Added: "+d+" read the PR p:4 to work\nUnder: Review id:4\n")
	assertEqual(t, "work.txt", e.readData("work.txt"), "Review id:4\n"+d+" read the PR p:4\nother p:9\n")

	// A number that names no task changes nothing.
	r = e.run("todo", "add", "--under", "9", "nowhere")
	if r.exitCode == 0 || !strings.Contains(r.stderr, "--under") {
		t.Errorf("--under 9: exit %d, stderr %q", r.exitCode, r.stderr)
	}
	if strings.Contains(e.readData("todo.txt"), "nowhere") {
		t.Error("--under 9 added the task")
	}
}

func TestSubtasks_NestAndUnnest(t *testing.T) {
	e := newEnv(t)
	e.writeData("todo.txt", launchTodo)

	r := e.mustRun("todo", "nest", "4", "0")
	assertEqual(t, "nest: stdout", r.stdout, "Nested: Pack the van p:1\nUnder:  (A) Plan the launch +work id:1\n")
	assertEqual(t, "after nest", e.readData("todo.txt"), launchTodo[:len(launchTodo)-len("Pack the van\n")]+"Pack the van p:1\n")

	// A loop is refused, and nothing changes.
	r = e.run("todo", "nest", "0", "3")
	assertEqual(t, "nest into its own subtree: stderr", r.stderr,
		"❌ nest task 0 under task 3: that would put the task below itself\n")
	if r.exitCode == 0 {
		t.Error("nest into its own subtree: exit 0")
	}

	r = e.mustRun("todo", "unnest", "2") // Book the venue, with Call three venues
	assertEqual(t, "unnest: stdout", r.stdout, "Unnested: Book the venue id:2\n")
	assertEqual(t, "after unnest", e.readData("todo.txt"),
		"(A) Plan the launch +work id:1\n"+
			"Write the announcement p:1 +work\n"+
			"Pack the van p:1\n"+
			"Book the venue id:2\n"+
			"Call three venues p:2\n")

	r = e.run("todo", "unnest", "0")
	assertEqual(t, "unnest a top-level task: stderr", r.stderr, "❌ unnest task 0: it's already a top-level task\n")
}

func TestSubtasks_MoveTakesTheSubtree(t *testing.T) {
	e := newEnv(t)
	e.writeData("todo.txt", launchTodo)
	e.writeData("work.txt", "Quarterly review id:1\nslides p:1\n")

	r := e.mustRun("todo", "move", "2", "work")
	assertEqual(t, "move: stdout", r.stdout,
		"Moved Book the venue id:2 from todo to work.\nMoved Call three venues p:2 from todo to work.\n")

	// Plan's id clashes with the review's: renumbered there, the link
	// from its subtask with it.
	r = e.mustRun("todo", "move", "0", "work")
	assertEqual(t, "move with a clash: stdout", r.stdout,
		"Moved (A) Plan the launch +work id:3 from todo to work.\nMoved Write the announcement p:3 +work from todo to work.\n")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "Pack the van\n")
	assertEqual(t, "work.txt", e.readData("work.txt"),
		"Quarterly review id:1\nslides p:1\nBook the venue id:2\nCall three venues p:2\n"+
			"(A) Plan the launch +work id:3\nWrite the announcement p:3 +work\n")
}

// "my list" names my_list's file: moving there is moving to the current
// context, refused, and the subtree stays.
func TestSubtasks_MoveToTheCurrentContextUnderAnotherSpelling(t *testing.T) {
	e := newEnv(t)
	e.mustRun("todo", "context", "my list")
	e.writeData("my_list.txt", "Parent id:1\nChild p:1\nOther\n")

	r := e.run("todo", "move", "0", "my list")
	if r.exitCode == 0 || !strings.Contains(r.stderr, "current context") {
		t.Errorf("move to \"my list\" from my_list: exit %d, stdout %q, stderr %q", r.exitCode, r.stdout, r.stderr)
	}
	assertEqual(t, "my_list.txt", e.readData("my_list.txt"), "Parent id:1\nChild p:1\nOther\n")
}

// --under takes a task number: given empty, or not a number, it's an
// error, and nothing is added (an empty one used to add a top-level
// task).
func TestSubtasks_UnderNeedsATaskNumber(t *testing.T) {
	e := newEnv(t)
	e.writeData("todo.txt", "Plan id:1\n")
	e.writeData("work.txt", "Review\n")
	for _, args := range [][]string{
		{"todo", "add", "--under", "", "meant as a subtask"},
		{"todo", "add", "--under=", "meant as a subtask"},
		{"todo", "add", "--under", "abc", "meant as a subtask"},
		{"todo", "add", "--under", "-1", "meant as a subtask"},
		{"todo", "add-to", "work", "--under", "", "meant as a subtask"},
	} {
		r := e.run(args...)
		if r.exitCode == 0 || !strings.Contains(r.stderr, "--under") || !strings.Contains(r.stderr, "task number") {
			t.Errorf("htt %s: exit %d, stderr %q", strings.Join(args, " "), r.exitCode, r.stderr)
		}
	}
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "Plan id:1\n")
	assertEqual(t, "work.txt", e.readData("work.txt"), "Review\n")

	// In the REPL too, and the next add isn't under anything.
	e.mustRunWithInput("add --under \"\" meant as a subtask\nadd Other\nq\n", "repl")
	assertEqual(t, "todo.txt after the REPL", e.readData("todo.txt"), "Plan id:1\n"+today()+" Other\n")
}

// workon logs the task's text, tags included, but not its id: and p:,
// which link to nothing in the time log. A task with no other text has
// nothing to log.
func TestSubtasks_WorkonLeavesTheLinksOut(t *testing.T) {
	e := newEnv(t)
	e.writeData("todo.txt", "Plan id:1\nWrite p:1 +work\np:1\n")
	assertEqual(t, "workon 1", e.mustRun("workon", "1").stdout, "Logging entry: Write +work\n")
	assertEqual(t, "workon 0", e.mustRun("workon", "0").stdout, "Logging entry: Plan\n")
	r := e.run("workon", "2")
	if r.exitCode == 0 || !strings.Contains(r.stderr, "no text besides its link") {
		t.Errorf("workon on a task with only its link: exit %d, stderr %q", r.exitCode, r.stderr)
	}
	log := e.readData(filepath.Join("timelogs", today()+".log"))
	if strings.Count(log, "\n") != 2 || strings.Contains(log, "id:") || strings.Contains(log, "p:") ||
		!strings.HasPrefix(log, "Write +work ts:") || !strings.Contains(log, "\nPlan ts:") {
		t.Errorf("time log:\n%s", log)
	}
}

// A done task moved back to the context it was completed in links up
// again: a parent keeps its id, its subtasks under it; a subtask keeps
// its p: when its parent is there.
func TestSubtasks_MoveADoneTaskBack(t *testing.T) {
	e := newEnv(t)
	e.writeData("todo.txt", "Plan id:1\nWrite p:1\nBook p:1\n")
	e.mustRun("todo", "do", "2") // Book
	e.mustRun("todo", "do", "0", "--leave-subtasks")
	d := today()

	e.mustRun("todo", "context", "done")
	r := e.mustRun("todo", "move", "1", "todo") // Plan
	assertEqual(t, "move Plan back: stdout", r.stdout, "Moved x "+d+" Plan id:1 context:todo from done to todo.\n")
	r = e.mustRun("todo", "move", "0", "todo") // Book
	assertEqual(t, "move Book back: stdout", r.stdout, "Moved x "+d+" Book p:1 context:todo from done to todo.\n")

	e.mustRun("todo", "context", "todo")
	r = e.mustRun("todo", "show")
	assertEqual(t, "show: stderr", r.stderr, "")
	assertInOrder(t, "show", r.stdout, "0 x "+d+" Plan id:1", "1   Write p:1", "2   x "+d+" Book p:1")
}

// A task whose only text is its p: would be an empty line without it:
// unnesting or moving it is refused, and nothing changes.
func TestSubtasks_ATaskWithOnlyItsLinkKeepsIt(t *testing.T) {
	e := newEnv(t)
	e.writeData("todo.txt", "Parent id:1\np:1\nOther\n")

	r := e.run("todo", "unnest", "1")
	assertEqual(t, "unnest: stderr", r.stderr,
		"❌ unnest task 1: it has no text besides its link, so without it the task would be empty: p:1\n")
	r = e.run("todo", "move", "1", "work")
	assertEqual(t, "move: stderr", r.stderr,
		"❌ move task: it has no text besides its link, so without it the task would be empty: p:1\n")
	if r.exitCode == 0 {
		t.Error("move: exit 0")
	}
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "Parent id:1\np:1\nOther\n")
	if e.dataExists("work.txt") {
		t.Errorf("work.txt: %q", e.readData("work.txt"))
	}
}

func TestSubtasks_DeleteTakesTheSubtree(t *testing.T) {
	e := newEnv(t)
	e.writeData("todo.txt", launchTodo)
	d := today()

	r := e.mustRun("todo", "delete", "2")
	assertEqual(t, "delete: stdout", r.stdout,
		"Deleted task: Book the venue p:1 id:2 archived-from:todo deleted-on:"+d+"\n"+
			"Deleted task: Call three venues p:2 archived-from:todo deleted-on:"+d+"\n")
	assertEqual(t, "todo.txt", e.readData("todo.txt"),
		"(A) Plan the launch +work id:1\nWrite the announcement p:1 +work\nPack the van\n")
	assertEqual(t, "archive.txt", e.readData("archive.txt"),
		"Book the venue p:1 id:2 archived-from:todo deleted-on:"+d+"\n"+
			"Call three venues p:2 archived-from:todo deleted-on:"+d+"\n")
}

// Numbers name the tasks in tree order, the order htt writes the file
// in, even in a file another tool left out of order: a command acts on
// the task shown with the number it's given.
func TestSubtasks_NumbersNameTheTaskShown(t *testing.T) {
	e := newEnv(t)
	// As topydo might leave it: a subtask after an unrelated task.
	e.writeData("todo.txt", "Plan id:1\nOther\nBook p:1\n")

	show := e.mustRun("todo", "show").stdout
	assertInOrder(t, "show", show, "0 Plan id:1", "1", "Book p:1", "2 Other")
	r := e.mustRun("todo", "do", "2")
	assertContains(t, "do 2", r.stdout, "Other")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "Plan id:1\nBook p:1\n")

	// Adding a subtask shifts the numbers below it; the next command
	// takes the numbers as they are now.
	e.mustRun("todo", "add", "Last")
	e.mustRun("todo", "add", "--under", "0", "Call")
	show = e.mustRun("todo", "show").stdout
	assertInOrder(t, "show", show, "0 Plan id:1", "1", "Book p:1", "2", "Call p:1", "3 "+today()+" Last")
	r = e.mustRun("todo", "delete", "3")
	assertContains(t, "delete 3", r.stdout, "Last")
}

// Changing a subtask's priority moves it among its siblings; the
// command names the task it changed, and the file says where it went.
func TestSubtasks_PriorityMovesASubtaskAmongItsSiblings(t *testing.T) {
	e := newEnv(t)
	e.writeData("todo.txt", launchTodo)

	r := e.mustRun("todo", "priority", "2", "B") // Book the venue
	assertEqual(t, "priority: stdout", r.stdout,
		"Before: Book the venue p:1 id:2\nAfter:  (B) Book the venue p:1 id:2\n")
	assertEqual(t, "todo.txt", e.readData("todo.txt"),
		"(A) Plan the launch +work id:1\n"+
			"(B) Book the venue p:1 id:2\n"+
			"Call three venues p:2\n"+
			"Write the announcement p:1 +work\n"+
			"Pack the van\n")

	r = e.mustRun("todo", "-", "1") // (B) Book the venue, down to (C)
	assertContains(t, "-: stdout", r.stdout, "After:  (C) Book the venue p:1 id:2")
	r = e.mustRun("todo", "+", "3") // Write the announcement, up to (C)
	assertEqual(t, "+: stdout", r.stdout,
		"Before: Write the announcement p:1 +work\nAfter:  (C) Write the announcement p:1 +work\n")
	assertEqual(t, "todo.txt", e.readData("todo.txt"),
		"(A) Plan the launch +work id:1\n"+
			"(C) Book the venue p:1 id:2\n"+
			"Call three venues p:2\n"+
			"(C) Write the announcement p:1 +work\n"+
			"Pack the van\n")
}

// Broken links warn on stderr, as dates htt can't read do: once a run,
// naming the file and line. The lines are left as they are.
func TestSubtasks_BrokenLinksWarnOnStderr(t *testing.T) {
	e := newEnv(t)
	broken := "loop a id:1 p:2\nloop b id:2 p:1\n\nlost p:9\ntwin id:5\ntwin too id:5\nchild p:5\nmany id:6\n" +
		"two parents p:6 p:1\nfine p:6\n"
	e.writeData("todo.txt", broken)
	todo := filepath.Join(e.dataDir, "todo.txt")
	warnings := "warning: " + todo + ":1: p:2 makes a loop (the task would be below itself), so it is shown at the top level\n" +
		"warning: " + todo + ":2: p:1 makes a loop (the task would be below itself), so it is shown at the top level\n" +
		"warning: " + todo + ":4: p:9 names no task, so the task is shown at the top level\n" +
		"warning: " + todo + ":5: id:5 is on more than one task, so its subtasks are shown at the top level\n" +
		"warning: " + todo + ":6: id:5 is on more than one task, so its subtasks are shown at the top level\n" +
		"warning: " + todo + ":9: has more than one p: (p:6 p:1); htt reads only the first\n"

	for _, args := range [][]string{{"todo", "show"}, {"todo"}, {"todo", "status"}, {"status"}} {
		r := e.mustRun(args...)
		assertEqual(t, "htt "+strings.Join(args, " ")+": stderr", r.stderr, warnings)
	}
	assertEqual(t, "todo.txt", e.readData("todo.txt"), broken)

	// Once a session in the REPL.
	r := e.mustRunWithInput("show\nshow\nq\n", "repl")
	assertEqual(t, "repl: stderr", r.stderr, warnings)

	// A typed p: naming nothing warns as it's written.
	e.writeData("todo.txt", "fine\n")
	r = e.mustRun("todo", "add", "stray p:7")
	assertEqual(t, "add: stderr", r.stderr, "warning: "+todo+":2: p:7 names no task, so the task is shown at the top level\n")

	// So does a link a typed task breaks: a parent replaced without its
	// id leaves its subtask naming nothing.
	e.writeData("todo.txt", "parent id:1\nchild p:1\n")
	r = e.mustRun("todo", "replace", "0", "parent, renamed")
	assertEqual(t, "replace: stderr", r.stderr, "warning: "+todo+":2: p:1 names no task, so the task is shown at the top level\n")
}

// A subtask whose parent was completed names a task in the done file: a
// done parent, not a broken link. A p: naming a task completed from
// another context is broken.
func TestSubtasks_ADoneParentIsNoBrokenLink(t *testing.T) {
	e := newEnv(t)
	e.writeData("todo.txt", "Book the venue id:2\nCall three venues p:2\nstray p:3\n")
	e.writeData("done.txt", "x 2026-01-01 elsewhere id:3 context:home\n")
	e.mustRun("todo", "do", "1") // Call: no subtasks
	r := e.mustRun("todo", "move", "0", "work")
	assertEqual(t, "move: stderr", r.stderr,
		"warning: "+filepath.Join(e.dataDir, "todo.txt")+":2: p:3 names no task, so the task is shown at the top level\n")
	e.writeData("todo.txt", "Book the venue id:2\nCall three venues p:2\n")
	e.writeData("done.txt", "x 2026-01-01 Book the venue id:2 context:todo\n")
	e.writeData("work.txt", "")
	r = e.mustRun("todo", "show")
	assertEqual(t, "show: stderr", r.stderr, "")
}
