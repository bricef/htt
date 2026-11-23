package domain_test

// Subtasks through the Context's persistent methods, against the memory
// repository (docs/design/subtasks.md).

import (
	"slices"
	"strings"
	"testing"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/storage"
)

// seed saves lines as the named context, as a file another tool wrote,
// and returns the repository.
func seed(t *testing.T, repo *storage.MemoryRepository, name string, lines ...string) {
	t.Helper()
	tasks := make([]*domain.Task, len(lines))
	for i, l := range lines {
		tasks[i] = mustTask(t, l)
	}
	if err := repo.Save(&domain.Context{Name: name, Tasks: tasks}); err != nil {
		t.Fatal(err)
	}
}

// rawsOf lists a context's lines as the repository holds them.
func rawsOf(t *testing.T, repo *storage.MemoryRepository, name string) []string {
	t.Helper()
	var out []string
	for _, task := range loadCtx(t, repo, name).Tasks {
		out = append(out, task.Raw)
	}
	return out
}

func wantLines(t *testing.T, label string, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s:\n  %s\nwant:\n  %s", label, strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// A subtask's priority change moves it among its siblings, and the
// change reports the task that changed. The top level sorts as it did
// before subtasks: by priority, after each priority change.
func TestContext_PriorityChangeMovesASubtaskAmongItsSiblings(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "todo",
		"Plan id:1",
		"(B) book p:1",
		"call p:1",
		"write p:1",
		"(C) later",
	)
	ctx := currentCtx(t, repo)
	before, after, err := ctx.SetPriority("2", "A") // call, after (B) book
	if err != nil {
		t.Fatal(err)
	}
	if before.Raw != "call p:1" || after.Raw != "(A) call p:1" {
		t.Errorf("reported %q → %q, want call p:1 → (A) call p:1", before.Raw, after.Raw)
	}
	wantLines(t, "after raising call", rawsOf(t, repo, "todo"),
		"(C) later",
		"Plan id:1",
		"(A) call p:1",
		"(B) book p:1",
		"write p:1",
	)

	ctx = currentCtx(t, repo)
	if _, after, err = ctx.DecreasePriority("3"); err != nil { // book
		t.Fatal(err)
	}
	if after.Raw != "(C) book p:1" {
		t.Errorf("reported %q, want (C) book p:1", after.Raw)
	}
	wantLines(t, "after lowering book", rawsOf(t, repo, "todo"),
		"(C) later",
		"Plan id:1",
		"(A) call p:1",
		"(C) book p:1",
		"write p:1",
	)
}

// A context loaded is numbered in tree order, so a number names the
// task shown with it, even in a file another tool left out of order.
func TestContext_NumbersFollowTheTree(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "todo",
		"call p:2",
		"plan id:1",
		"other",
		"book p:1 id:2",
	)
	ctx := currentCtx(t, repo)
	wantLines(t, "loaded", func() []string {
		var out []string
		for i, task := range ctx.Tasks {
			if task.Line != i {
				t.Errorf("%q has Line %d at %d", task.Raw, task.Line, i)
			}
			out = append(out, task.Raw)
		}
		return out
	}(), "plan id:1", "book p:1 id:2", "call p:2", "other")

	// 2 is the task shown as 2: call.
	done, err := ctx.Complete("2", domain.SubtasksAsk)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(done[0].Raw, "call p:2") {
		t.Errorf("completed %q, want call", done[0].Raw)
	}
	wantLines(t, "left", rawsOf(t, repo, "todo"), "plan id:1", "book p:1 id:2", "other")
}
