package domain

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bricef/htt/internal/vars"
	"github.com/spf13/viper"
)

// newCtxWithTasks builds a Context populated with raw task lines, using
// the test helper mustTask (which fails the test on a malformed entry).
func newCtxWithTasks(t *testing.T, name string, raws ...string) *Context {
	t.Helper()
	c := &Context{Name: name, Tasks: []*Task{}}
	for _, raw := range raws {
		c.Tasks = append(c.Tasks, mustTask(t, raw))
	}
	return c
}

func TestContext_Equals(t *testing.T) {
	a := &Context{Name: "todo"}
	b := &Context{Name: "todo"}
	c := &Context{Name: "work"}

	if !a.Equals(b) {
		t.Errorf("contexts with same name should be Equal")
	}
	if a.Equals(c) {
		t.Errorf("contexts with different names should not be Equal")
	}
}

func TestContext_GetTaskById(t *testing.T) {
	ctx := newCtxWithTasks(t, "todo", "alpha", "beta", "gamma")

	got, err := ctx.GetTaskById(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Raw != "beta" {
		t.Errorf("got %q, want beta", got.Raw)
	}

	if _, err := ctx.GetTaskById(99); err == nil {
		t.Errorf("expected out-of-range error")
	}
	if _, err := ctx.GetTaskById(-1); err == nil {
		t.Errorf("expected negative-index error")
	}
}

func TestContext_GetTaskById_EmptyContext(t *testing.T) {
	ctx := newCtxWithTasks(t, "todo")
	if _, err := ctx.GetTaskById(0); err == nil {
		t.Errorf("expected error on empty context")
	}
}

func TestContext_GetTaskByStrId(t *testing.T) {
	ctx := newCtxWithTasks(t, "todo", "a", "b")

	got, err := ctx.GetTaskByStrId("0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Raw != "a" {
		t.Errorf("got %q, want a", got.Raw)
	}

	if _, err := ctx.GetTaskByStrId("not-a-number"); err == nil {
		t.Errorf("expected parse error")
	}
}

func TestContext_GetTaskIndex(t *testing.T) {
	ctx := newCtxWithTasks(t, "todo", "a", "b", "c")

	target := ctx.Tasks[1]
	idx, err := ctx.GetTaskIndex(target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 1 {
		t.Errorf("got %d, want 1", idx)
	}

	other := mustTask(t, "not in context")
	if _, err := ctx.GetTaskIndex(other); err == nil {
		t.Errorf("expected error for task not in context")
	}
}

func TestContext_Replace(t *testing.T) {
	ctx := newCtxWithTasks(t, "todo", "old", "keep")
	old := ctx.Tasks[0]
	replacement := mustTask(t, "new")

	if err := ctx.Replace(old, replacement); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Tasks[0].Raw != "new" {
		t.Errorf("Tasks[0] = %q, want new", ctx.Tasks[0].Raw)
	}
	if ctx.Tasks[1].Raw != "keep" {
		t.Errorf("Tasks[1] should be unchanged, got %q", ctx.Tasks[1].Raw)
	}
}

func TestContext_Replace_TaskNotPresent(t *testing.T) {
	ctx := newCtxWithTasks(t, "todo", "a")
	other := mustTask(t, "not there")
	if err := ctx.Replace(other, mustTask(t, "x")); err == nil {
		t.Errorf("expected error replacing absent task")
	}
}

func TestContext_Search(t *testing.T) {
	ctx := newCtxWithTasks(t, "todo", "buy milk", "buy bread", "call alice")

	matches := ctx.Search(func(t *Task) bool {
		return len(t.Raw) >= 3 && t.Raw[:3] == "buy"
	})
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
	if matches[0].Raw != "buy milk" || matches[1].Raw != "buy bread" {
		t.Errorf("unexpected match order: %q, %q", matches[0].Raw, matches[1].Raw)
	}
}

func TestContext_Add_DoesNotTouchDisk(t *testing.T) {
	// Step 10 invariant: Context.Add only mutates the in-memory slice;
	// it must not produce a file at Filepath(). Legacy Add called Sync
	// internally, which was the bug fueling the read-writes-back issue
	// and the source of the duplicated-Sync calls in CLI/TUI.
	dir := t.TempDir()
	viper.Set(vars.ConfigKeyDataDir, dir)

	ctx := NewContext("invariant")
	expectedPath := filepath.Join(dir, "invariant.txt")

	ctx.Add(mustTask(t, "test"))

	if _, err := os.Stat(expectedPath); !os.IsNotExist(err) {
		t.Errorf("Add should not write to disk; file %s exists (err=%v)", expectedPath, err)
	}
}

func TestContext_Remove_DoesNotTouchDisk(t *testing.T) {
	dir := t.TempDir()
	viper.Set(vars.ConfigKeyDataDir, dir)

	ctx := newCtxWithTasks(t, "invariant", "task")
	expectedPath := filepath.Join(dir, "invariant.txt")

	if err := ctx.Remove(ctx.Tasks[0]); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := os.Stat(expectedPath); !os.IsNotExist(err) {
		t.Errorf("Remove should not write to disk; file %s exists (err=%v)", expectedPath, err)
	}
}

func TestContext_Sort_ByPriority(t *testing.T) {
	ctx := newCtxWithTasks(t, "todo",
		"no priority task",
		"(A) urgent",
		"(C) low",
		"(B) medium",
	)
	ctx.Sort()

	wantOrder := []string{"(A) urgent", "(B) medium", "(C) low", "no priority task"}
	for i, want := range wantOrder {
		if ctx.Tasks[i].Raw != want {
			t.Errorf("Tasks[%d] = %q, want %q", i, ctx.Tasks[i].Raw, want)
		}
	}
}
