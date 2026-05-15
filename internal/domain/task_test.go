package domain

import (
	"errors"
	"testing"
	"time"
)

// mustTask parses raw as a Task or fails the test. Helper for the many
// assertion sites below that don't care about the parse-error path.
func mustTask(t *testing.T, raw string) *Task {
	t.Helper()
	task, err := NewTask(raw)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", raw, err)
	}
	return task
}

// mustAnnotate calls Annotate or fails the test.
func mustAnnotate(t *testing.T, task *Task, key, value string) *Task {
	t.Helper()
	out, err := task.Annotate(key, value)
	if err != nil {
		t.Fatalf("Annotate(%q,%q): %v", key, value, err)
	}
	return out
}

func equal(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("['%s' != '%s']", got, want)
	}
}

func TestTaskCreation(t *testing.T) {
	t.Run("Entry parsed for simple tasks", func(t *testing.T) {
		equal(t, mustTask(t, "Hello World").Entry(), "Hello World")
	})

	t.Run("A single tag is parsed", func(t *testing.T) {
		equal(t, mustTask(t, "Hello World @foo").Tags["@"][0], "@foo")
	})

	t.Run("Annotations are absent from parsed entry", func(t *testing.T) {
		equal(t, mustTask(t, "Hello tag:val World").Entry(), "Hello World")
	})

	t.Run("Annotation values available", func(t *testing.T) {
		equal(t, mustTask(t, "Hello tag:val World").Annotations["tag"], "val")
	})

	t.Run("Can change the priority", func(t *testing.T) {
		equal(t, mustTask(t, "(B) hello world").setPriority("A").ConsoleString(), "(A) hello world")
	})

	t.Run("Can increase the priority", func(t *testing.T) {
		equal(t, mustTask(t, "(B) hello world").increasePriority().ConsoleString(), "(A) hello world")
	})

	t.Run("Can decrease the priority", func(t *testing.T) {
		equal(t, mustTask(t, "(A) hello world").decreasePriority().ConsoleString(), "(B) hello world")
	})

	// NOTE: priorities = ["A", "B", "C", ""]. Decreasing an unknown priority
	// (anything outside A-C) resets to "" (no priority) rather than clamping
	// at the lowest valid letter. This test pins that current behavior.
	t.Run("Decreasing an unknown priority resets to no priority", func(t *testing.T) {
		equal(t, mustTask(t, "(F) hello world").decreasePriority().ConsoleString(), "hello world")
	})

	t.Run("Decrease from no priority stays at no priority", func(t *testing.T) {
		equal(t, mustTask(t, "hello world").decreasePriority().ConsoleString(), "hello world")
	})

	t.Run("Can't increase the priority past maximum", func(t *testing.T) {
		equal(t, mustTask(t, "(A) hello world").increasePriority().ConsoleString(), "(A) hello world")
	})

	t.Run("Increase from no priority steps up to C", func(t *testing.T) {
		equal(t, mustTask(t, "hello world").increasePriority().ConsoleString(), "(C) hello world")
	})

	t.Run("Can add an annotation", func(t *testing.T) {
		equal(t, mustAnnotate(t, mustTask(t, "hello"), "test", "123").ConsoleString(), "hello test:123")
	})

	t.Run("Can remove an annotation", func(t *testing.T) {
		equal(t, mustTask(t, "hello test:123").RemoveAnnotation("test").ConsoleString(), "hello")
	})
}

// Pre-existing parser bug: NewTask was silently failing to populate
// CompletedOn / CreatedOn even when the dates were present at the
// expected position. The Maybe wrapper in parser.go returned the inner
// Token's node (named COMPLETIONDATE / CREATIONDATE) instead of one
// renamed to the Maybe's name (COMPLETEDAT / CREATEDAT), so the
// QueryOne("COMPLETEDAT") / QueryOne("CREATEDAT") in task.go found
// nothing. These tests pin the corrected behaviour.

func TestNewTask_ParsesCompletedOn(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want time.Time
	}{
		{
			name: "completion mark plus date",
			raw:  "x 2026-05-16 ship the auth refactor",
			want: time.Date(2026, 5, 16, 0, 0, 0, 0, time.Local),
		},
		{
			name: "completion mark, date, then priority",
			raw:  "x 2026-05-16 (A) urgent thing context:work",
			want: time.Date(2026, 5, 16, 0, 0, 0, 0, time.Local),
		},
		{
			name: "completion mark, date, then created-on",
			raw:  "x 2026-05-16 2026-05-10 something annotated:value",
			want: time.Date(2026, 5, 16, 0, 0, 0, 0, time.Local),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			task := mustTask(t, c.raw)
			if !task.Completed {
				t.Errorf("Completed = false, want true")
			}
			if !task.CompletedOn.Equal(c.want) {
				t.Errorf("CompletedOn = %v, want %v", task.CompletedOn, c.want)
			}
		})
	}
}

func TestNewTask_ParsesCreatedOn(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want time.Time
	}{
		{
			name: "bare creation date",
			raw:  "2026-05-10 wrote the plan doc",
			want: time.Date(2026, 5, 10, 0, 0, 0, 0, time.Local),
		},
		{
			name: "priority then creation date",
			raw:  "(B) 2026-05-10 wrote the plan doc",
			want: time.Date(2026, 5, 10, 0, 0, 0, 0, time.Local),
		},
		{
			name: "completion + completed-on + creation date",
			raw:  "x 2026-05-16 2026-05-10 wrote the plan doc context:work",
			want: time.Date(2026, 5, 10, 0, 0, 0, 0, time.Local),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			task := mustTask(t, c.raw)
			if !task.CreatedOn.Equal(c.want) {
				t.Errorf("CreatedOn = %v, want %v", task.CreatedOn, c.want)
			}
		})
	}
}

func TestNewTask_EmptyReturnsErr(t *testing.T) {
	_, err := NewTask("")
	if !errors.Is(err, ErrEmptyTask) {
		t.Errorf("NewTask(\"\") err = %v, want ErrEmptyTask", err)
	}
}

func TestAnnotate_RejectsInvalidKey(t *testing.T) {
	task := mustTask(t, "hello")
	// Keys starting with @, +, #, or : are reserved by todo.txt syntax.
	_, err := task.Annotate("@bad", "value")
	if !errors.Is(err, ErrInvalidAnnotationKey) {
		t.Errorf("Annotate(@bad,value) err = %v, want ErrInvalidAnnotationKey", err)
	}
}

func TestAnnotate_RejectsInvalidValue(t *testing.T) {
	task := mustTask(t, "hello")
	// Values containing a backtick are rejected by kvpairValueRegexp.
	_, err := task.Annotate("ok", "bad`value")
	if !errors.Is(err, ErrInvalidAnnotationValue) {
		t.Errorf("Annotate(ok,bad`value) err = %v, want ErrInvalidAnnotationValue", err)
	}
}
