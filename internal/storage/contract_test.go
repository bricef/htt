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

// runRepositoryContract exercises the behaviors every domain.Repository
// implementation must satisfy. file_test.go runs the same suite against
// the file-backed impl.
func runRepositoryContract(t *testing.T, newRepo func(t *testing.T) domain.Repository) {
	t.Helper()

	t.Run("Context returns empty for an unknown name", func(t *testing.T) {
		r := newRepo(t)
		ctx, err := r.Context("never-saved")
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

	t.Run("Save then Context round-trips", func(t *testing.T) {
		r := newRepo(t)
		original := &domain.Context{
			Name: "todo",
			Tasks: []*domain.Task{
				mustTask(t, "buy milk"),
				mustTask(t, "(A) call alice"),
			},
		}
		if err := r.Save(original); err != nil {
			t.Fatalf("save: %v", err)
		}
		loaded, err := r.Context("todo")
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

	t.Run("Save preserves task order", func(t *testing.T) {
		r := newRepo(t)
		raws := []string{"first", "second", "third", "fourth"}
		tasks := make([]*domain.Task, len(raws))
		for i, raw := range raws {
			tasks[i] = mustTask(t, raw)
		}
		if err := r.Save(&domain.Context{Name: "ordered", Tasks: tasks}); err != nil {
			t.Fatalf("save: %v", err)
		}
		loaded, err := r.Context("ordered")
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		for i, want := range raws {
			if loaded.Tasks[i].Raw != want {
				t.Errorf("Tasks[%d] = %q, want %q", i, loaded.Tasks[i].Raw, want)
			}
		}
	})

	t.Run("Save overwrites prior state", func(t *testing.T) {
		r := newRepo(t)
		_ = r.Save(&domain.Context{
			Name:  "todo",
			Tasks: []*domain.Task{mustTask(t, "v1 task")},
		})
		_ = r.Save(&domain.Context{
			Name:  "todo",
			Tasks: []*domain.Task{mustTask(t, "v2 task a"), mustTask(t, "v2 task b")},
		})
		loaded, err := r.Context("todo")
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

	t.Run("ContextNames returns all saved context names", func(t *testing.T) {
		r := newRepo(t)
		names, err := r.ContextNames()
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(names) != 0 {
			t.Errorf("fresh repo should list 0 contexts, got %v", names)
		}

		_ = r.Save(&domain.Context{Name: "todo", Tasks: []*domain.Task{mustTask(t, "a")}})
		_ = r.Save(&domain.Context{Name: "work", Tasks: []*domain.Task{mustTask(t, "b")}})
		_ = r.Save(&domain.Context{Name: "done", Tasks: []*domain.Task{mustTask(t, "x c")}})

		names, err = r.ContextNames()
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		got := map[string]bool{}
		for _, n := range names {
			got[n] = true
		}
		for _, want := range []string{"todo", "work", "done"} {
			if !got[want] {
				t.Errorf("ContextNames missing %q; got %v", want, names)
			}
		}
	})

	t.Run("Contexts returns every persisted context with tasks loaded", func(t *testing.T) {
		r := newRepo(t)

		// Fresh store: empty list, not nil.
		all, err := r.Contexts()
		if err != nil {
			t.Fatalf("Contexts: %v", err)
		}
		if all == nil {
			t.Fatalf("Contexts on empty store returned nil, want empty slice")
		}
		if len(all) != 0 {
			t.Errorf("fresh repo Contexts = %v, want empty", all)
		}

		_ = r.Save(&domain.Context{Name: "todo", Tasks: []*domain.Task{mustTask(t, "a1"), mustTask(t, "a2")}})
		_ = r.Save(&domain.Context{Name: "work", Tasks: []*domain.Task{mustTask(t, "b1")}})

		all, err = r.Contexts()
		if err != nil {
			t.Fatalf("Contexts: %v", err)
		}
		byName := map[string]*domain.Context{}
		for _, c := range all {
			byName[c.Name] = c
		}
		if got := byName["todo"]; got == nil || len(got.Tasks) != 2 {
			t.Errorf("todo context not loaded with tasks; got %#v", got)
		}
		if got := byName["work"]; got == nil || len(got.Tasks) != 1 {
			t.Errorf("work context not loaded with tasks; got %#v", got)
		}
	})

	t.Run("CurrentContextName defaults to todo", func(t *testing.T) {
		r := newRepo(t)
		name, err := r.CurrentContextName()
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if name != domain.DefaultContextName {
			t.Errorf("default current context = %q, want %q", name, domain.DefaultContextName)
		}
	})

	t.Run("CurrentContext defaults to todo and is loaded", func(t *testing.T) {
		r := newRepo(t)

		// No pointer set, no tasks saved: empty todo context.
		ctx, err := r.CurrentContext()
		if err != nil {
			t.Fatalf("CurrentContext on fresh repo: %v", err)
		}
		if ctx.Name != domain.DefaultContextName {
			t.Errorf("Name = %q, want %q", ctx.Name, domain.DefaultContextName)
		}
		if len(ctx.Tasks) != 0 {
			t.Errorf("fresh CurrentContext should have no tasks, got %v", ctx.Tasks)
		}

		// After SetCurrent + Save, CurrentContext returns that context with tasks.
		if err := r.Save(&domain.Context{Name: "work", Tasks: []*domain.Task{mustTask(t, "ship")}}); err != nil {
			t.Fatalf("save work: %v", err)
		}
		if err := r.SetCurrent("work"); err != nil {
			t.Fatalf("SetCurrent: %v", err)
		}
		ctx, err = r.CurrentContext()
		if err != nil {
			t.Fatalf("CurrentContext after switch: %v", err)
		}
		if ctx.Name != "work" {
			t.Errorf("Name = %q, want work", ctx.Name)
		}
		if len(ctx.Tasks) != 1 || ctx.Tasks[0].Raw != "ship" {
			t.Errorf("tasks = %v, want one ship task", ctx.Tasks)
		}
	})

	t.Run("SetCurrent persists and round-trips", func(t *testing.T) {
		r := newRepo(t)
		if err := r.SetCurrent("work"); err != nil {
			t.Fatalf("set: %v", err)
		}
		name, err := r.CurrentContextName()
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if name != "work" {
			t.Errorf("current = %q, want work", name)
		}
	})

	t.Run("SetCurrent sanitizes non-word characters", func(t *testing.T) {
		// Sanitization (non-word → underscore) moved from usecase.SwitchContext
		// into Repository.SetCurrent so every caller benefits. Pinning the
		// behavior here keeps the contract honest after the move.
		r := newRepo(t)
		if err := r.SetCurrent("hello world!"); err != nil {
			t.Fatalf("SetCurrent: %v", err)
		}
		name, err := r.CurrentContextName()
		if err != nil {
			t.Fatalf("CurrentContextName: %v", err)
		}
		if name != "hello_world_" {
			t.Errorf("persisted name = %q, want hello_world_", name)
		}
	})

	t.Run("Empty names are rejected", func(t *testing.T) {
		r := newRepo(t)
		if err := r.Save(&domain.Context{Name: ""}); !errors.Is(err, domain.ErrInvalidContextName) {
			t.Errorf("Save(\"\"): err = %v, want ErrInvalidContextName", err)
		}
		if _, err := r.Context(""); !errors.Is(err, domain.ErrInvalidContextName) {
			t.Errorf("Context(\"\"): err = %v, want ErrInvalidContextName", err)
		}
		if err := r.SetCurrent(""); !errors.Is(err, domain.ErrInvalidContextName) {
			t.Errorf("SetCurrent(\"\"): err = %v, want ErrInvalidContextName", err)
		}
	})

	t.Run("Save does not alias caller's task slice", func(t *testing.T) {
		r := newRepo(t)
		input := &domain.Context{
			Name:  "todo",
			Tasks: []*domain.Task{mustTask(t, "original")},
		}
		if err := r.Save(input); err != nil {
			t.Fatalf("save: %v", err)
		}

		input.Tasks = append(input.Tasks, mustTask(t, "mutation after save"))

		loaded, err := r.Context("todo")
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if len(loaded.Tasks) != 1 {
			t.Errorf("post-save mutation leaked into stored state; len = %d, want 1", len(loaded.Tasks))
		}
	})
}

func TestMemoryRepository_Contract(t *testing.T) {
	runRepositoryContract(t, func(t *testing.T) domain.Repository {
		return NewMemoryRepository()
	})
}
