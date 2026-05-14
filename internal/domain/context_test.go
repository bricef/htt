package domain

import (
	"testing"
)

// newCtxWithTasks builds a Context populated with raw task lines, bypassing
// the I/O-bound Add() (which calls Sync). This is the pure-value setup we
// want for testing the non-I/O methods.
func newCtxWithTasks(name string, raws ...string) *Context {
	c := &Context{Name: name, Tasks: []*Task{}}
	for _, raw := range raws {
		c.Tasks = append(c.Tasks, NewTask(raw))
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
	ctx := newCtxWithTasks("todo", "alpha", "beta", "gamma")

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
	ctx := newCtxWithTasks("todo")
	if _, err := ctx.GetTaskById(0); err == nil {
		t.Errorf("expected error on empty context")
	}
}

func TestContext_GetTaskByStrId(t *testing.T) {
	ctx := newCtxWithTasks("todo", "a", "b")

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
	ctx := newCtxWithTasks("todo", "a", "b", "c")

	target := ctx.Tasks[1]
	idx, err := ctx.GetTaskIndex(target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 1 {
		t.Errorf("got %d, want 1", idx)
	}

	other := NewTask("not in context")
	if _, err := ctx.GetTaskIndex(other); err == nil {
		t.Errorf("expected error for task not in context")
	}
}

func TestContext_Replace(t *testing.T) {
	ctx := newCtxWithTasks("todo", "old", "keep")
	old := ctx.Tasks[0]
	replacement := NewTask("new")

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
	ctx := newCtxWithTasks("todo", "a")
	other := NewTask("not there")
	if err := ctx.Replace(other, NewTask("x")); err == nil {
		t.Errorf("expected error replacing absent task")
	}
}

func TestContext_Search(t *testing.T) {
	ctx := newCtxWithTasks("todo", "buy milk", "buy bread", "call alice")

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

func TestContext_Sort_ByPriority(t *testing.T) {
	ctx := newCtxWithTasks("todo",
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
