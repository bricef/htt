package domain

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
)

// Rewriting a task (a priority change, completion, an annotation)
// changes what was asked and nothing else: every other word stays
// where it was written, annotations included, in their order, and an
// annotation repeated with the same key keeps every occurrence.

// words splits a line on the parser's whitespace, keeping order.
func words(line string) []string {
	return strings.FieldsFunc(line, func(r rune) bool { return strings.ContainsRune(wordSpace, r) })
}

func TestRewrite_KeepsWordOrder(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 8))
	for range 3000 {
		raw := randomLine(r)
		task := mustTask(t, raw)
		priority := []string{"A", "B", "C"}[r.IntN(3)]

		// The priority word is replaced where it stands (after "x" on a
		// done task) or inserted there; nothing else moves.
		want := words(raw)
		at := 0
		if task.Completed {
			at = 1
		}
		if task.Priority != "" {
			want[at] = "(" + priority + ")"
		} else {
			want = slices.Insert(want, at, "("+priority+")")
		}

		task.setPriority(priority)
		if got := words(task.Raw); !slices.Equal(got, want) {
			t.Fatalf("setting priority %s on\n  %q\nwrote\n  %q\nwant\n  %q", priority, raw, task.Raw, strings.Join(want, " "))
		}
	}
}

func TestComplete_KeepsWordOrder(t *testing.T) {
	r := rand.New(rand.NewPCG(9, 10))
	when := time.Date(2026, 5, 22, 9, 0, 0, 0, time.Local)
	for range 3000 {
		raw := randomLine(r)
		task := mustTask(t, raw)
		if task.Completed {
			continue
		}
		// "x" first, then any priority, then the completion date; the
		// rest as written, and the new context annotation last.
		all := words(raw)
		want := []string{"x"}
		if task.Priority != "" {
			want, all = append(want, all[0]), all[1:]
		}
		want = append(want, "2026-05-22")
		want = append(want, all...)
		want = append(want, "context:todo")

		if _, err := task.markCompleted(&Context{Name: "todo"}, when); err != nil {
			t.Fatalf("markCompleted(%q): %v", raw, err)
		}
		if got := words(task.Raw); !slices.Equal(got, want) {
			t.Fatalf("completing\n  %q\nwrote\n  %q\nwant\n  %q", raw, task.Raw, strings.Join(want, " "))
		}
	}
}

func TestRewrite_TheKnownCases(t *testing.T) {
	cases := []struct {
		name, raw string
		rewrite   func(*Task) *Task
		want      string
	}{
		{"an annotation stays where it was written",
			"call due:2026-05-20 mum about +garden", func(t *Task) *Task { return t.setPriority("A") },
			"(A) call due:2026-05-20 mum about +garden"},
		{"annotations keep their order",
			"pay k1:a k2:b k3:c k4:d k5:e k6:f now", func(t *Task) *Task { return t.setPriority("B") },
			"(B) pay k1:a k2:b k3:c k4:d k5:e k6:f now"},
		{"a repeated key keeps every occurrence",
			"tag a:1 then a:2", func(t *Task) *Task { return t.setPriority("A") },
			"(A) tag a:1 then a:2"},
		{"a date after an annotation stays text",
			"due:x 2026-05-20 buy", func(t *Task) *Task { return t.setPriority("A") },
			"(A) due:x 2026-05-20 buy"},
		{"lowering a priority moves nothing",
			"(B) pay ref:42 rent", func(t *Task) *Task { return t.decreasePriority() },
			"(C) pay ref:42 rent"},
		{"Annotate changes an annotation in place",
			"pay rent due:2026-05-01 now", func(t *Task) *Task { return mustAnnotateT(t, "due", "2026-06-01") },
			"pay rent due:2026-06-01 now"},
		{"Annotate changes the last of a repeated key, the one Annotations shows",
			"a:1 b a:2 c", func(t *Task) *Task { return mustAnnotateT(t, "a", "3") },
			"a:1 b a:3 c"},
		{"Annotate adds a new key at the end",
			"pay rent", func(t *Task) *Task { return mustAnnotateT(t, "due", "2026-06-01") },
			"pay rent due:2026-06-01"},
		{"RemoveAnnotation removes every occurrence and nothing else moves",
			"a:1 b due:2026-05-01 a:2 c", func(t *Task) *Task { return t.RemoveAnnotation("a") },
			"b due:2026-05-01 c"},
		{"StampCreated dates a new task without moving its annotations",
			"(A) call due:friday mum", func(t *Task) *Task { return t.StampCreated(time.Date(2026, 5, 22, 0, 0, 0, 0, time.Local)) },
			"(A) 2026-05-22 call due:friday mum"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := mustTask(t, tc.raw)
			got := tc.rewrite(task)
			if got.Raw != tc.want {
				t.Fatalf("%q became %q, want %q", tc.raw, got.Raw, tc.want)
			}
			// The rewrite reads back as what the task now holds.
			again := mustTask(t, got.Raw)
			if again.Entry() != got.Entry() || again.Priority != got.Priority || !again.CreatedOn.Equal(got.CreatedOn) ||
				len(again.Annotations) != len(got.Annotations) {
				t.Errorf("%q reads back as entry %q priority %q created %v annotations %v; the task held entry %q priority %q created %v annotations %v",
					got.Raw, again.Entry(), again.Priority, again.CreatedOn, again.Annotations,
					got.Entry(), got.Priority, got.CreatedOn, got.Annotations)
			}
			for k, v := range got.Annotations {
				if again.Annotations[k] != v {
					t.Errorf("annotation %s reads back as %q, the task held %q", k, again.Annotations[k], v)
				}
			}
		})
	}
}

func TestDelete_AnnotationsGoAtTheEndInOrder(t *testing.T) {
	task := mustTask(t, "pay due:2026-05-01 rent")
	if _, err := task.markDeleted(&Context{Name: "todo"}, time.Date(2026, 5, 22, 0, 0, 0, 0, time.Local)); err != nil {
		t.Fatalf("markDeleted: %v", err)
	}
	if want := "pay due:2026-05-01 rent archived-from:todo deleted-on:2026-05-22"; task.Raw != want {
		t.Errorf("deleting wrote %q, want %q", task.Raw, want)
	}
}

func mustAnnotateT(t *Task, key, value string) *Task {
	out, err := t.Annotate(key, value)
	if err != nil {
		panic(err)
	}
	return out
}
