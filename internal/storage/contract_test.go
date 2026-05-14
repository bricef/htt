package storage

import (
	"errors"
	"testing"

	"github.com/bricef/htt/internal/domain"
)

// mustTask is the test helper for parsing task lines; failures abort the test.
func mustTask(t *testing.T, raw string) *domain.Task {
	t.Helper()
	task, err := domain.NewTask(raw)
	if err != nil {
		t.Fatalf("domain.NewTask(%q): %v", raw, err)
	}
	return task
}

// runRepositoryContract exercises the behaviors every Repository
// implementation must satisfy. file.go's TestFileRepository_Contract will
// invoke the same suite against the file-backed impl in Step 6.
func runRepositoryContract(t *testing.T, newRepo func(t *testing.T) Repository) {
	t.Helper()

	t.Run("LoadContext returns empty for an unknown name", func(t *testing.T) {
		r := newRepo(t)
		ctx, err := r.LoadContext("never-saved")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ctx.Name != "never-saved" {
			t.Errorf("Name = %q, want never-saved", ctx.Name)
		}
		if len(ctx.Tasks) != 0 {
			t.Errorf("Tasks = %v, want empty", ctx.Tasks)
		}
	})

	t.Run("SaveContext then LoadContext round-trips", func(t *testing.T) {
		r := newRepo(t)
		original := &domain.Context{
			Name: "todo",
			Tasks: []*domain.Task{
				mustTask(t,"buy milk"),
				mustTask(t,"(A) call alice"),
			},
		}
		if err := r.SaveContext(original); err != nil {
			t.Fatalf("save: %v", err)
		}
		loaded, err := r.LoadContext("todo")
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if loaded.Name != "todo" {
			t.Errorf("Name = %q, want todo", loaded.Name)
		}
		if len(loaded.Tasks) != 2 {
			t.Fatalf("len(Tasks) = %d, want 2", len(loaded.Tasks))
		}
		if loaded.Tasks[0].Raw != "buy milk" {
			t.Errorf("Tasks[0].Raw = %q, want buy milk", loaded.Tasks[0].Raw)
		}
		if loaded.Tasks[1].Raw != "(A) call alice" {
			t.Errorf("Tasks[1].Raw = %q, want (A) call alice", loaded.Tasks[1].Raw)
		}
	})

	t.Run("SaveContext preserves task order", func(t *testing.T) {
		r := newRepo(t)
		raws := []string{"first", "second", "third", "fourth"}
		tasks := make([]*domain.Task, len(raws))
		for i, raw := range raws {
			tasks[i] = mustTask(t,raw)
		}
		if err := r.SaveContext(&domain.Context{Name: "ordered", Tasks: tasks}); err != nil {
			t.Fatalf("save: %v", err)
		}
		loaded, err := r.LoadContext("ordered")
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		for i, want := range raws {
			if loaded.Tasks[i].Raw != want {
				t.Errorf("Tasks[%d] = %q, want %q", i, loaded.Tasks[i].Raw, want)
			}
		}
	})

	t.Run("SaveContext overwrites prior state", func(t *testing.T) {
		r := newRepo(t)
		_ = r.SaveContext(&domain.Context{
			Name:  "todo",
			Tasks: []*domain.Task{mustTask(t,"v1 task")},
		})
		_ = r.SaveContext(&domain.Context{
			Name:  "todo",
			Tasks: []*domain.Task{mustTask(t,"v2 task a"), mustTask(t,"v2 task b")},
		})
		loaded, err := r.LoadContext("todo")
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if len(loaded.Tasks) != 2 {
			t.Fatalf("len(Tasks) = %d after overwrite, want 2", len(loaded.Tasks))
		}
		if loaded.Tasks[0].Raw != "v2 task a" {
			t.Errorf("expected v2 contents, got %q", loaded.Tasks[0].Raw)
		}
	})

	t.Run("ListContexts returns all saved context names", func(t *testing.T) {
		r := newRepo(t)
		names, err := r.ListContexts()
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(names) != 0 {
			t.Errorf("fresh repo should list 0 contexts, got %v", names)
		}

		_ = r.SaveContext(&domain.Context{Name: "todo", Tasks: []*domain.Task{mustTask(t,"a")}})
		_ = r.SaveContext(&domain.Context{Name: "work", Tasks: []*domain.Task{mustTask(t,"b")}})
		_ = r.SaveContext(&domain.Context{Name: "done", Tasks: []*domain.Task{mustTask(t,"x c")}})

		names, err = r.ListContexts()
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		got := map[string]bool{}
		for _, n := range names {
			got[n] = true
		}
		for _, want := range []string{"todo", "work", "done"} {
			if !got[want] {
				t.Errorf("ListContexts missing %q; got %v", want, names)
			}
		}
	})

	t.Run("GetCurrentContextName defaults to todo", func(t *testing.T) {
		r := newRepo(t)
		name, err := r.GetCurrentContextName()
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if name != DefaultContextName {
			t.Errorf("default current context = %q, want %q", name, DefaultContextName)
		}
	})

	t.Run("SetCurrentContextName persists and round-trips", func(t *testing.T) {
		r := newRepo(t)
		if err := r.SetCurrentContextName("work"); err != nil {
			t.Fatalf("set: %v", err)
		}
		name, err := r.GetCurrentContextName()
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if name != "work" {
			t.Errorf("current = %q, want work", name)
		}
	})

	t.Run("Empty names are rejected", func(t *testing.T) {
		r := newRepo(t)
		if err := r.SaveContext(&domain.Context{Name: ""}); !errors.Is(err, ErrInvalidContextName) {
			t.Errorf("SaveContext(\"\"): err = %v, want ErrInvalidContextName", err)
		}
		if _, err := r.LoadContext(""); !errors.Is(err, ErrInvalidContextName) {
			t.Errorf("LoadContext(\"\"): err = %v, want ErrInvalidContextName", err)
		}
		if err := r.SetCurrentContextName(""); !errors.Is(err, ErrInvalidContextName) {
			t.Errorf("SetCurrentContextName(\"\"): err = %v, want ErrInvalidContextName", err)
		}
	})

	t.Run("SaveContext does not alias caller's task slice", func(t *testing.T) {
		r := newRepo(t)
		input := &domain.Context{
			Name:  "todo",
			Tasks: []*domain.Task{mustTask(t,"original")},
		}
		if err := r.SaveContext(input); err != nil {
			t.Fatalf("save: %v", err)
		}

		input.Tasks = append(input.Tasks, mustTask(t,"mutation after save"))

		loaded, err := r.LoadContext("todo")
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if len(loaded.Tasks) != 1 {
			t.Errorf("post-save mutation leaked into stored state; len = %d, want 1", len(loaded.Tasks))
		}
	})
}

func TestMemoryRepository_Contract(t *testing.T) {
	runRepositoryContract(t, func(t *testing.T) Repository {
		return NewMemoryRepository()
	})
}
