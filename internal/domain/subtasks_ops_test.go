package domain_test

// Adding, arranging, moving, deleting and completing subtasks
// (docs/design/subtasks.md, "Behaviour").

import (
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/bricef/htt/internal/domain"
)

var today = time.Now().Format("2006-01-02")

func rawsOfTasks(tasks []*domain.Task) []string {
	var out []string
	for _, t := range tasks {
		out = append(out, t.Raw)
	}
	return out
}

func treeProblems(tree *domain.Tree) []string {
	var out []string
	for _, p := range tree.Problems {
		out = append(out, strconv.Itoa(p.Index)+": "+p.Message)
	}
	return out
}

// --- Adding under a task ----------------------------------------------------

func TestContext_AddSubtask(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "work",
		"Plan the launch +work",
		"orphan p:1", // p:1 holds 1: the parent gets 2
		"Other",
	)
	ctx := loadCtx(t, repo, "work")
	if err := ctx.AddSubtask("0", mustTask(t, "Write the announcement")); err != nil {
		t.Fatal(err)
	}
	wantLines(t, "first subtask", rawsOf(t, repo, "work"),
		"Plan the launch +work id:2",
		today+" Write the announcement p:2",
		"orphan p:1",
		"Other",
	)

	// The next goes after the parent's last descendant, or at its
	// priority's place; the parent keeps its id.
	ctx = loadCtx(t, repo, "work")
	if err := ctx.AddSubtask("1", mustTask(t, "Find a photo")); err != nil {
		t.Fatal(err)
	}
	ctx = loadCtx(t, repo, "work")
	if err := ctx.AddSubtask("0", mustTask(t, "Book the venue")); err != nil {
		t.Fatal(err)
	}
	ctx = loadCtx(t, repo, "work")
	if err := ctx.AddSubtask("0", mustTask(t, "(A) Set the date")); err != nil {
		t.Fatal(err)
	}
	wantLines(t, "more subtasks", rawsOf(t, repo, "work"),
		"Plan the launch +work id:2",
		"(A) "+today+" Set the date p:2",
		today+" Write the announcement p:2 id:3",
		today+" Find a photo p:3",
		today+" Book the venue p:2",
		"orphan p:1",
		"Other",
	)
}

func TestContext_AddSubtask_Refused(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "work", "one id:1", "two id:1")
	seed(t, repo, domain.DoneContextName, "x done id:4")

	ctx := loadCtx(t, repo, "work")
	if err := ctx.AddSubtask("0", mustTask(t, "child")); !errors.Is(err, domain.ErrSharedID) {
		t.Errorf("under a task whose id another holds: %v, want ErrSharedID", err)
	}
	if err := ctx.AddSubtask("5", mustTask(t, "child")); err == nil {
		t.Error("under a task that isn't there: no error")
	}
	done := loadCtx(t, repo, domain.DoneContextName)
	if err := done.AddSubtask("0", mustTask(t, "child")); !errors.Is(err, domain.ErrNotATree) {
		t.Errorf("in the done file: %v, want ErrNotATree", err)
	}
	wantLines(t, "work", rawsOf(t, repo, "work"), "one id:1", "two id:1")
	wantLines(t, "done", rawsOf(t, repo, domain.DoneContextName), "x done id:4")
}

// --- Nesting and unnesting ---------------------------------------------------

func TestContext_Nest(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "work",
		"Book the venue id:1",
		"Call three venues p:1",
		"Plan the launch",
		"Write p:3",
		"misc id:3",
	)
	// Loaded: Book, Call, Plan, misc, Write. misc holds 3 and Write
	// names it: Plan gets 2.
	ctx := loadCtx(t, repo, "work")
	moved, parent, err := ctx.Nest("0", "2") // Book, with Call, under Plan
	if err != nil {
		t.Fatal(err)
	}
	if moved.Raw != "Book the venue id:1 p:2" || parent.Raw != "Plan the launch id:2" {
		t.Errorf("reported %q under %q", moved.Raw, parent.Raw)
	}
	wantLines(t, "nested", rawsOf(t, repo, "work"),
		"Plan the launch id:2",
		"Book the venue id:1 p:2",
		"Call three venues p:1",
		"misc id:3",
		"Write p:3",
	)

	// Under a task with subtasks: last among those of its priority,
	// with its own subtree.
	ctx = loadCtx(t, repo, "work")
	if _, _, err := ctx.Nest("3", "0"); err != nil { // misc, with Write, under Plan
		t.Fatal(err)
	}
	wantLines(t, "under a parent with subtasks", rawsOf(t, repo, "work"),
		"Plan the launch id:2",
		"Book the venue id:1 p:2",
		"Call three venues p:1",
		"misc id:3 p:2",
		"Write p:3",
	)

	// Up a level: Call from under Book to under Plan, after misc.
	ctx = loadCtx(t, repo, "work")
	if _, _, err := ctx.Nest("2", "0"); err != nil {
		t.Fatal(err)
	}
	wantLines(t, "up a level", rawsOf(t, repo, "work"),
		"Plan the launch id:2",
		"Book the venue id:1 p:2",
		"misc id:3 p:2",
		"Write p:3",
		"Call three venues p:2",
	)
}

func TestContext_Nest_Refused(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "work",
		"Plan id:1",
		"Book p:1 id:2",
		"Call p:2",
		"twin id:7",
		"twin too id:7",
	)
	cases := []struct {
		name, task, parent string
		want               error
	}{
		{"under itself", "0", "0", domain.ErrLoop},
		{"under its subtask", "0", "1", domain.ErrLoop},
		{"under a deeper subtask", "0", "2", domain.ErrLoop},
		{"under a task whose id another holds", "2", "3", domain.ErrSharedID},
	}
	for _, tc := range cases {
		ctx := loadCtx(t, repo, "work")
		if _, _, err := ctx.Nest(tc.task, tc.parent); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v, want %v", tc.name, err, tc.want)
		}
	}
	wantLines(t, "unchanged", rawsOf(t, repo, "work"),
		"Plan id:1", "Book p:1 id:2", "Call p:2", "twin id:7", "twin too id:7")
}

func TestContext_Nest_ATaskWithSeveralParentsKeepsOne(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "work", "a id:1", "b id:2", "c p:1 x p:2 y")
	ctx := loadCtx(t, repo, "work") // a, c, b
	if _, _, err := ctx.Nest("1", "2"); err != nil {
		t.Fatal(err)
	}
	wantLines(t, "one p: left", rawsOf(t, repo, "work"), "a id:1", "b id:2", "c p:2 x y")
}

func TestContext_Unnest(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "work",
		"Plan id:1",
		"Book p:1 id:2",
		"Call p:2 id:3",
		"Ring Ann p:3",
		"Pay p:2",
		"Write p:1",
		"Other",
	)
	ctx := loadCtx(t, repo, "work")
	moved, err := ctx.Unnest("2") // Call, with Ring Ann
	if err != nil {
		t.Fatal(err)
	}
	if moved.Raw != "Call id:3" {
		t.Errorf("reported %q", moved.Raw)
	}
	// After its top-level ancestor's subtree; Book keeps its id.
	wantLines(t, "unnested", rawsOf(t, repo, "work"),
		"Plan id:1",
		"Book p:1 id:2",
		"Pay p:2",
		"Write p:1",
		"Call id:3",
		"Ring Ann p:3",
		"Other",
	)

	ctx = loadCtx(t, repo, "work")
	if _, err := ctx.Unnest("6"); !errors.Is(err, domain.ErrTopLevel) {
		t.Errorf("a top-level task: %v, want ErrTopLevel", err)
	}
}

func TestContext_Unnest_ATaskShownAtTheTopLevelLosesItsBrokenLink(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "work", "lost p:9", "Other")
	ctx := loadCtx(t, repo, "work")
	if _, err := ctx.Unnest("0"); err != nil {
		t.Fatal(err)
	}
	wantLines(t, "unnested", rawsOf(t, repo, "work"), "lost", "Other")
}

// The TUI's Tab: under the task above at the same level.
func TestContext_NestUnderPrevious(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "work",
		"Plan id:1",
		"Book p:1 id:2",
		"Call p:2",
		"(A) Write p:1",
		"Launch",
		"Party",
	)
	// Loaded: Plan, (A) Write, Book, Call, Launch, Party.
	cases := []struct {
		name, task string
		want       []string
	}{
		{"a top-level task, under the top-level task above, past its subtasks", "4", []string{
			"Plan id:1", "(A) Write p:1", "Book p:1 id:2", "Call p:2", "Launch p:1", "Party",
		}},
		{"a subtask, under the subtask above, with its subtree", "2", []string{
			"Plan id:1", "(A) Write p:1 id:3", "Book p:3 id:2", "Call p:2", "Launch p:1", "Party",
		}},
		{"a top-level task under one with a deep subtree: last of its priority", "5", []string{
			"Plan id:1", "(A) Write p:1 id:3", "Book p:3 id:2", "Call p:2", "Launch p:1", "Party p:1",
		}},
	}
	for _, tc := range cases {
		ctx := loadCtx(t, repo, "work")
		moved, err := ctx.NestUnderPrevious(tc.task)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if moved != ctx.Tasks[moved.Line] {
			t.Errorf("%s: the moved task isn't numbered where it is", tc.name)
		}
		wantLines(t, tc.name, rawsOf(t, repo, "work"), tc.want...)
	}

	// Plan; (A) Write, Plan's first subtask; Call, Book's only one.
	for _, first := range []string{"0", "1", "3"} {
		ctx := loadCtx(t, repo, "work")
		if _, err := ctx.NestUnderPrevious(first); !errors.Is(err, domain.ErrNothingAbove) {
			t.Errorf("task %s, the first at its level: %v, want ErrNothingAbove", first, err)
		}
	}
}

// The TUI's Shift-Tab: a level up, after the old parent's subtree.
func TestContext_UnnestOneLevel(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "work",
		"Plan id:1",
		"Book p:1 id:2",
		"Call p:2 id:3",
		"Ring Ann p:3",
		"Pay p:2",
		"Write p:1",
		"(A) Other",
	)
	ctx := loadCtx(t, repo, "work")
	moved, err := ctx.UnnestOneLevel("2") // Call, with Ring Ann, under Plan
	if err != nil {
		t.Fatal(err)
	}
	if moved.Raw != "Call p:1 id:3" {
		t.Errorf("reported %q", moved.Raw)
	}
	if moved != ctx.Tasks[moved.Line] {
		t.Error("the moved task isn't numbered where it is")
	}
	wantLines(t, "a level up", rawsOf(t, repo, "work"),
		"Plan id:1",
		"Book p:1 id:2",
		"Pay p:2",
		"Call p:1 id:3",
		"Ring Ann p:3",
		"Write p:1",
		"(A) Other",
	)

	// A subtask of a top-level task: top-level, right after its old
	// parent's subtree, whatever its priority.
	ctx = loadCtx(t, repo, "work")
	if _, err := ctx.UnnestOneLevel("1"); err != nil { // Book, with Pay
		t.Fatal(err)
	}
	wantLines(t, "to the top level", rawsOf(t, repo, "work"),
		"Plan id:1",
		"Call p:1 id:3",
		"Ring Ann p:3",
		"Write p:1",
		"Book id:2",
		"Pay p:2",
		"(A) Other",
	)

	// Priority order among the new siblings: (A) urgent, under Call,
	// comes up a level ahead of Call and Write.
	ctx = loadCtx(t, repo, "work")
	if err := ctx.AddSubtask("1", mustTask(t, "(A) 2026-01-01 urgent")); err != nil {
		t.Fatal(err)
	}
	ctx = loadCtx(t, repo, "work") // Plan, Call, (A) urgent, Ring Ann, …
	if _, err := ctx.UnnestOneLevel("2"); err != nil {
		t.Fatal(err)
	}
	wantLines(t, "in priority order", rawsOf(t, repo, "work"),
		"Plan id:1",
		"(A) 2026-01-01 urgent p:1",
		"Call p:1 id:3",
		"Ring Ann p:3",
		"Write p:1",
		"Book id:2",
		"Pay p:2",
		"(A) Other",
	)

	ctx = loadCtx(t, repo, "work")
	if _, err := ctx.UnnestOneLevel("0"); !errors.Is(err, domain.ErrTopLevel) {
		t.Errorf("a top-level task: %v, want ErrTopLevel", err)
	}
}

// --- Moving and deleting subtrees --------------------------------------------

func TestContext_Move_TakesTheSubtreeAndRenumbersClashes(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "work",
		"Plan id:1",
		"Book p:1 id:2",
		"Call p:2",
		"Write p:1",
		"Launch id:7",
		"Party p:7",
	)
	seed(t, repo, "home",
		"Garden id:1",
		"Weed p:1",
		"orphan p:3", // p:3 holds 3: no moved id may take it
	)
	ctx := loadCtx(t, repo, "work")     // Plan, Book, Call, Write, Launch, Party
	moved, err := ctx.Move("1", "home") // Book, with Call
	if err != nil {
		t.Fatal(err)
	}
	if got := rawsOfTasks(moved); !slices.Equal(got, []string{"Book id:2", "Call p:2"}) {
		t.Errorf("moved %q", got)
	}
	wantLines(t, "work", rawsOf(t, repo, "work"), "Plan id:1", "Write p:1", "Launch id:7", "Party p:7")
	wantLines(t, "home", rawsOf(t, repo, "home"), "Garden id:1", "Weed p:1", "orphan p:3", "Book id:2", "Call p:2")

	// Plan's id clashes with Garden's: renumbered to home's smallest
	// free id, its subtasks' links with it.
	ctx = loadCtx(t, repo, "work")
	if _, err := ctx.Move("0", "home"); err != nil {
		t.Fatal(err)
	}
	wantLines(t, "home", rawsOf(t, repo, "home"),
		"Garden id:1", "Weed p:1", "orphan p:3", "Book id:2", "Call p:2", "Plan id:4", "Write p:4")
}

// An id the target uses only in a p: (naming a done parent there, say)
// clashes too: kept, the moved task would adopt that task.
func TestContext_Move_AnIDTheTargetNamesInAPClashes(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "work", "Launch id:7", "Party p:7")
	seed(t, repo, "home", "left over p:7")
	ctx := loadCtx(t, repo, "work")
	if _, err := ctx.Move("0", "home"); err != nil {
		t.Fatal(err)
	}
	wantLines(t, "home", rawsOf(t, repo, "home"), "left over p:7", "Launch id:1", "Party p:1")
}

func TestContext_Move_IntoTheDoneFileLeavesTheLinesAlone(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "work", "Plan id:1", "Book p:1 id:2", "Call p:2")
	seed(t, repo, domain.DoneContextName, "x 2026-01-01 old id:1")
	ctx := loadCtx(t, repo, "work")
	if _, err := ctx.Move("1", domain.DoneContextName); err != nil {
		t.Fatal(err)
	}
	wantLines(t, "done", rawsOf(t, repo, domain.DoneContextName), "x 2026-01-01 old id:1", "Book p:1 id:2", "Call p:2")
	wantLines(t, "work", rawsOf(t, repo, "work"), "Plan id:1")
}

func TestContext_Delete_ArchivesTheSubtree(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "work", "Plan id:1", "Book p:1 id:2", "Call p:2", "Write p:1", "Other")
	ctx := loadCtx(t, repo, "work")
	gone, err := ctx.Delete("1")
	if err != nil {
		t.Fatal(err)
	}
	suffix := " archived-from:work deleted-on:" + today
	if got := rawsOfTasks(gone); !slices.Equal(got, []string{"Book p:1 id:2" + suffix, "Call p:2" + suffix}) {
		t.Errorf("deleted %q", got)
	}
	wantLines(t, "work", rawsOf(t, repo, "work"), "Plan id:1", "Write p:1", "Other")
	wantLines(t, "archive", rawsOf(t, repo, domain.ArchiveContextName), "Book p:1 id:2"+suffix, "Call p:2"+suffix)

	// The archive isn't a tree: deleting there removes one line.
	archive := loadCtx(t, repo, domain.ArchiveContextName)
	if gone, err := archive.Delete("0"); err != nil || len(gone) != 1 {
		t.Fatalf("delete in the archive: %v, %d tasks", err, len(gone))
	}
	wantLines(t, "archive", rawsOf(t, repo, domain.ArchiveContextName), "Call p:2"+suffix)
}

// --- Completing ----------------------------------------------------------------

func TestContext_Complete_WithOpenSubtasks(t *testing.T) {
	lines := []string{
		"Plan id:1",
		"Book p:1 id:2",
		"Call p:2",
		"x 2026-01-01 Rang Ann p:2",
		"Write p:1",
		"Other",
	}
	done := " context:work"

	t.Run("asks first: refused, nothing changed", func(t *testing.T) {
		repo := newRepo(t)
		seed(t, repo, "work", lines...)
		ctx := loadCtx(t, repo, "work")
		open, err := ctx.OpenSubtasks("0")
		if err != nil || len(open) != 3 {
			t.Fatalf("OpenSubtasks: %d, %v; want 3 (Book, Call, Write)", len(open), err)
		}
		if _, err := ctx.Complete("0", domain.SubtasksAsk); !errors.Is(err, domain.ErrOpenSubtasks) {
			t.Errorf("Complete: %v, want ErrOpenSubtasks", err)
		}
		wantLines(t, "work", rawsOf(t, repo, "work"), lines...)
		wantLines(t, "done", rawsOf(t, repo, domain.DoneContextName))
	})

	t.Run("x: the task alone; its subtasks stay, naming it", func(t *testing.T) {
		repo := newRepo(t)
		seed(t, repo, "work", lines...)
		ctx := loadCtx(t, repo, "work")
		got, err := ctx.Complete("1", domain.SubtasksLeave)
		if err != nil || len(got) != 1 {
			t.Fatalf("Complete: %d tasks, %v", len(got), err)
		}
		wantLines(t, "work", rawsOf(t, repo, "work"),
			"Plan id:1", "Write p:1", "Call p:2", "x 2026-01-01 Rang Ann p:2", "Other")
		wantLines(t, "done", rawsOf(t, repo, domain.DoneContextName), "x "+today+" Book p:1 id:2"+done)

		// Book is in the done file: its subtasks show at the top level,
		// marked, without a problem.
		tree, err := loadCtx(t, repo, "work").Tree()
		if err != nil {
			t.Fatal(err)
		}
		if n := tree.Nodes[2]; n.Parent != nil || !n.DoneParent || len(tree.Problems) != 0 {
			t.Errorf("Call: parent %v, done parent %v, problems %v", n.Parent, n.DoneParent, tree.Problems)
		}
	})

	t.Run("A: the task and every open task below it", func(t *testing.T) {
		repo := newRepo(t)
		seed(t, repo, "work", lines...)
		ctx := loadCtx(t, repo, "work")
		got, err := ctx.Complete("0", domain.SubtasksComplete)
		if err != nil || len(got) != 4 {
			t.Fatalf("Complete: %d tasks, %v", len(got), err)
		}
		wantLines(t, "work", rawsOf(t, repo, "work"), "x 2026-01-01 Rang Ann p:2", "Other")
		wantLines(t, "done", rawsOf(t, repo, domain.DoneContextName),
			"x "+today+" Plan id:1"+done,
			"x "+today+" Book p:1 id:2"+done,
			"x "+today+" Call p:2"+done,
			"x "+today+" Write p:1"+done,
		)
	})

	t.Run("no open subtasks: as before, whatever the choice", func(t *testing.T) {
		repo := newRepo(t)
		seed(t, repo, "work", "Call id:1", "x 2026-01-01 done p:1", "Other")
		ctx := loadCtx(t, repo, "work")
		if got, err := ctx.Complete("0", domain.SubtasksAsk); err != nil || len(got) != 1 {
			t.Fatalf("Complete: %d tasks, %v", len(got), err)
		}
		wantLines(t, "work", rawsOf(t, repo, "work"), "x 2026-01-01 done p:1", "Other")
	})
}

// A done parent is one completed from this context: the same id in the
// done file from another context names nothing here.
func TestContext_Tree_DoneParentsAreTheContexts(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "work", "child p:4", "other p:5")
	seed(t, repo, domain.DoneContextName,
		"x 2026-01-01 parent id:4 context:work",
		"x 2026-01-01 elsewhere id:5 context:home",
	)
	tree, err := loadCtx(t, repo, "work").Tree()
	if err != nil {
		t.Fatal(err)
	}
	if !tree.Nodes[0].DoneParent || tree.Nodes[1].DoneParent {
		t.Errorf("done parents: %v %v, want true false", tree.Nodes[0].DoneParent, tree.Nodes[1].DoneParent)
	}
	if got := treeProblems(tree); !slices.Equal(got, []string{"1: p:5 names no task, so the task is shown at the top level"}) {
		t.Errorf("problems: %q", got)
	}

	// The done file and the archive are flat.
	tree, err = loadCtx(t, repo, domain.DoneContextName).Tree()
	if err != nil || len(tree.Roots) != 2 || len(tree.Problems) != 0 {
		t.Errorf("done: %d roots, problems %v, err %v", len(tree.Roots), tree.Problems, err)
	}
}
