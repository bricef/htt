package domain

import (
	"testing"
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

// stubRepository is a placeholder Repository for the NewContext wiring
// test. Methods panic if invoked: the only thing under test here is
// identity, not behavior.
type stubRepository struct{}

func (*stubRepository) Context(string) (*Context, error)    { panic("stub") }
func (*stubRepository) Contexts() ([]*Context, error)       { panic("stub") }
func (*stubRepository) ContextNames() ([]string, error)     { panic("stub") }
func (*stubRepository) CurrentContext() (*Context, error)   { panic("stub") }
func (*stubRepository) CurrentContextName() (string, error) { panic("stub") }
func (*stubRepository) SetCurrent(string) error             { panic("stub") }
func (*stubRepository) Save(*Context) error                 { panic("stub") }

func TestNewContext_InjectsRepo(t *testing.T) {
	// Step 2 invariant: NewContext stores the supplied Repository on the
	// Context. Without this, persistent methods (AddTask, Complete, …)
	// would nil-deref. The repo field is package-private, so this
	// wiring check must live in the domain package.
	stub := &stubRepository{}
	ctx := NewContext(stub, "wired")

	if ctx.repo != Repository(stub) {
		t.Errorf("ctx.repo did not get the supplied repo")
	}
	if ctx.Name != "wired" {
		t.Errorf("ctx.Name = %q, want wired", ctx.Name)
	}
	if len(ctx.Tasks) != 0 {
		t.Errorf("ctx.Tasks should start empty, got %v", ctx.Tasks)
	}
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

