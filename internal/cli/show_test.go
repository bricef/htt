package cli

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bricef/htt/internal/domain"
)

// showNow anchors the due-date filters: 2026-05-16, mid-afternoon in
// the machine's own zone, so "today" is 2026-05-16 wherever the tests
// run.
var showNow = time.Date(2026, 5, 16, 15, 0, 0, 0, time.Local)

// viewTasks parses raws into tasks whose Line is their position in
// the list, as the file repository would number them.
func viewTasks(t *testing.T, raws ...string) []*domain.Task {
	t.Helper()
	tasks := make([]*domain.Task, len(raws))
	for i, raw := range raws {
		task, err := domain.NewTask(raw)
		if err != nil {
			t.Fatalf("NewTask(%q): %v", raw, err)
		}
		task.Line = i
		tasks[i] = task
	}
	return tasks
}

func viewEntries(tasks []*domain.Task) []string {
	out := make([]string, len(tasks))
	for i, task := range tasks {
		out[i] = task.Entry()
	}
	return out
}

func viewLines(tasks []*domain.Task) []int {
	out := make([]int, len(tasks))
	for i, task := range tasks {
		out[i] = task.Line
	}
	return out
}

func TestParseSortKey_Orders(t *testing.T) {
	tasks := viewTasks(t,
		"(B) 2026-05-03 bravo due:2026-05-20",
		"2026-05-01 nopri due:2026-05-18",
		"(A) charlie",                      // no created date, no due date
		"(C) 2026-05-02 delta due:someday", // malformed due counts as none
		"(A) 2026-05-04 echo due:2026-05-19",
	)

	cases := []struct {
		key  string
		want []string
	}{
		// Ties (charlie, echo both A) keep file order.
		{"priority", []string{"charlie", "echo", "bravo", "delta", "nopri"}},
		{"-priority", []string{"nopri", "delta", "bravo", "charlie", "echo"}},
		// Tasks without the key stay last in both directions.
		{"created", []string{"nopri", "delta", "bravo", "echo", "charlie"}},
		{"-created", []string{"echo", "bravo", "delta", "nopri", "charlie"}},
		{"due", []string{"nopri", "echo", "bravo", "charlie", "delta"}},
		{"-due", []string{"bravo", "echo", "nopri", "charlie", "delta"}},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			compare, err := parseSortKey(tc.key)
			if err != nil {
				t.Fatalf("parseSortKey(%q): %v", tc.key, err)
			}
			got := slices.Clone(tasks)
			slices.SortStableFunc(got, compare)
			if !slices.Equal(viewEntries(got), tc.want) {
				t.Errorf("--sort %s = %v, want %v", tc.key, viewEntries(got), tc.want)
			}
		})
	}
}

func TestParseSortKey_RejectsUnknownKeys(t *testing.T) {
	for _, key := range []string{"", "-", "entry", "--due", "Priority"} {
		_, err := parseSortKey(key)
		if err == nil {
			t.Errorf("parseSortKey(%q): expected error, got nil", key)
			continue
		}
		if !strings.Contains(err.Error(), "invalid --sort") {
			t.Errorf("parseSortKey(%q) error = %q, want it to mention invalid --sort", key, err)
		}
	}
}

func TestParseFilter_Matches(t *testing.T) {
	tasks := viewTasks(t,
		"(A) late due:2026-05-15",
		"(B) today due:2026-05-16 @home",
		"sixth due:2026-05-22 +home",
		"(A) seventh due:2026-05-23 #home",
		"undated +htt @homework",
		"garbled due:friday",
	)

	cases := []struct {
		expr string
		want []int
	}{
		{"overdue", []int{0}},
		// [today, today+7d): today counts, the seventh day out does not.
		{"due-soon", []int{1, 2}},
		{"no-due", []int{4, 5}},
		// A bare name matches any sigil but only the whole tag.
		{"has-tag:home", []int{1, 2, 3}},
		{"has-tag:+home", []int{2}},
		{"has-tag:htt", []int{4}},
		{"priority:A", []int{0, 3}},
		{"priority:b", []int{1}},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			predicate, err := parseFilter(tc.expr, showNow)
			if err != nil {
				t.Fatalf("parseFilter(%q): %v", tc.expr, err)
			}
			got := []int{}
			for _, task := range tasks {
				if predicate(task) {
					got = append(got, task.Line)
				}
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("--filter %s matched %v, want %v", tc.expr, got, tc.want)
			}
		})
	}
}

func TestParseFilter_RejectsUnknownExpressions(t *testing.T) {
	for _, expr := range []string{"", "late", "has-tag:", "priority:", "priority:D", "priority:AB"} {
		_, err := parseFilter(expr, showNow)
		if err == nil {
			t.Errorf("parseFilter(%q): expected error, got nil", expr)
			continue
		}
		if !strings.Contains(err.Error(), "invalid --filter") {
			t.Errorf("parseFilter(%q) error = %q, want it to mention invalid --filter", expr, err)
		}
	}
}

func TestBuildView_AndsFiltersThenSortsWithoutTouchingContext(t *testing.T) {
	ctx := &domain.Context{Name: "work", Tasks: viewTasks(t,
		"(A) 2026-05-01 first due:2026-05-17",
		"(C) 2026-05-03 wrong-priority due:2026-05-18",
		"(A) 2026-05-03 third due:2026-05-19",
		"(A) 2026-05-02 second due:2026-05-20",
		"(A) 2026-05-04 not-soon due:2026-06-30",
	)}

	view, err := buildView(ctx, "-created", []string{"priority:A", "due-soon"}, showNow)
	if err != nil {
		t.Fatalf("buildView: %v", err)
	}
	if got, want := viewLines(view), []int{2, 3, 0}; !slices.Equal(got, want) {
		t.Errorf("view lines = %v, want %v", got, want)
	}
	if got, want := viewLines(ctx.Tasks), []int{0, 1, 2, 3, 4}; !slices.Equal(got, want) {
		t.Errorf("ctx.Tasks reordered to %v, want file order %v", got, want)
	}
}

// seedShowContext stores a few tasks in the memory repo's current
// context so the show command has something to sort and filter.
func seedShowContext(t *testing.T) {
	t.Helper()
	repo := withMemoryRepo(t)
	ctx := domain.NewContext(repo, "todo")
	ctx.Tasks = viewTasks(t,
		"(A) 2026-05-01 ship it due:2026-05-17",
		"2026-05-02 buy milk @home",
		"(B) tidy up",
	)
	if err := repo.Save(ctx); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func TestCobra_ShowWithSortAndFilter_Runs(t *testing.T) {
	for _, args := range [][]string{
		{"todo", "show", "--sort", "-created"},
		{"todo", "show", "--filter", "no-due", "--filter", "has-tag:home", "--sort", "priority"},
		{"todo", "show", "--filter", "priority:C"}, // matches nothing
	} {
		seedShowContext(t)
		if err := runCobra(t, args...); err != nil {
			t.Errorf("%v: unexpected error: %v", args, err)
		}
	}
}

func TestCobra_ShowRejectsBadFlags(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"todo", "show", "--filter", "someday"}, `invalid --filter "someday"`},
		{[]string{"todo", "show", "--sort", "size"}, `invalid --sort "size"`},
	}
	for _, tc := range cases {
		seedShowContext(t)
		err := runCobra(t, tc.args...)
		if err == nil {
			t.Errorf("%v: expected error, got nil", tc.args)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: error = %q, want it to contain %q", tc.args, err, tc.want)
		}
	}
}
