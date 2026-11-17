package domain

import (
	"maps"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

// A todo.txt line opens with a preamble: x (done), a priority, then
// dates. When a rewrite leaves less in front of the task's text (a
// priority removed, a leading annotation removed), the text's first
// word could be read as part of that preamble: `(C) x foo` lowered
// would read as done, `(C) (B) foo` as priority B. A date in front
// stops that, and it's idiomatic todo.txt: so the rewrite gives the
// task today's date in the empty date slot, and nothing else changes.

func today() string { return time.Now().Format("2006-01-02") }

func TestRewrite_DatesATaskRatherThanChangeWhatItSays(t *testing.T) {
	lower := func(t *Task) *Task { return t.decreasePriority() }
	unprioritise := func(t *Task) *Task { return t.setPriority("") }
	remove := func(key string) func(*Task) *Task {
		return func(t *Task) *Task { return t.RemoveAnnotation(key) }
	}
	cases := []struct {
		name, raw string
		rewrite   func(*Task) *Task
		want      string
	}{
		{"lowering (C) x foo keeps x as text", "(C) x foo", lower, today() + " x foo"},
		{"lowering (C) (B) foo keeps (B) as text", "(C) (B) foo", lower, today() + " (B) foo"},
		{"a task with a created date needs nothing", "(C) 2026-01-01 x foo", lower, "2026-01-01 x foo"},
		{"nothing to protect, nothing added", "(C) plain task", lower, "plain task"},
		{"x alone is a word, not the done mark", "(C) x", lower, "x"},
		{"removing a leading annotation keeps x as text", "due:x x foo", remove("due"), today() + " x foo"},
		{"removing a leading annotation keeps (B) as text", "ref:1 (B) foo", remove("ref"), today() + " (B) foo"},
		{"removing a leading annotation keeps a date as text", "due:x 2026-05-20 buy", remove("due"), today() + " 2026-05-20 buy"},
		{"a done task's (B) stays text", "x (C) (B) foo", unprioritise, "x " + today() + " (B) foo"},
		{"a done task can need both its dates", "x due:x 2026-05-20 foo", remove("due"), "x " + today() + " " + today() + " 2026-05-20 foo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := mustTask(t, tc.raw)
			entry, done := task.Entry(), task.Completed
			got := tc.rewrite(task)
			if got.Raw != tc.want {
				t.Errorf("%q became %q, want %q", tc.raw, got.Raw, tc.want)
			}
			again := mustTask(t, got.Raw)
			if again.Entry() != entry || again.Completed != done || again.Priority != got.Priority {
				t.Errorf("%q reads back as entry %q done %v priority %q; want entry %q done %v priority %q",
					got.Raw, again.Entry(), again.Completed, again.Priority, entry, done, got.Priority)
			}
		})
	}
}

func TestRewrite_KeepsTextDoneStateAndPriority(t *testing.T) {
	r := rand.New(rand.NewPCG(11, 12))
	for range 3000 {
		raw := randomLine(r)
		task := mustTask(t, raw)
		entry, done := task.Entry(), task.Completed

		// A random rewrite: any priority, none included, or removing
		// one of the task's annotations.
		what, added := "", map[string]bool{today(): true}
		if keys := slices.Sorted(maps.Keys(task.Annotations)); len(keys) > 0 && r.IntN(2) == 0 {
			key := keys[r.IntN(len(keys))]
			what = "removing " + key
			task.RemoveAnnotation(key)
		} else {
			p := []string{"A", "B", "C", ""}[r.IntN(4)]
			what = "setting priority " + p
			task.setPriority(p)
			added["("+p+")"] = true
		}

		again := mustTask(t, task.Raw)
		if again.Entry() != entry || again.Completed != done || again.Priority != task.Priority ||
			!again.CompletedOn.Equal(task.CompletedOn) || !again.CreatedOn.Equal(task.CreatedOn) ||
			!maps.Equal(again.Annotations, task.Annotations) {
			t.Fatalf("%s on\n  %q\nwrote\n  %q\nwhich reads back as entry %q done %v priority %q; the task holds entry %q done %v priority %q",
				what, raw, task.Raw, again.Entry(), again.Completed, again.Priority, entry, done, task.Priority)
		}
		// The only words a rewrite adds are the new priority and dates.
		before := wordCounts(raw)
		for w, n := range wordCounts(task.Raw) {
			if n > before[w] && !added[w] {
				t.Fatalf("%s on %q wrote %q, adding %q", what, raw, task.Raw, w)
			}
		}
	}
}
