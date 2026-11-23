package domain_test

import (
	"errors"
	"testing"

	"github.com/bricef/htt/internal/domain"
)

// Following a task a command waited on the user for (follow.go).

func TestFindTask(t *testing.T) {
	tasks := []*domain.Task{mustTask(t, "a"), mustTask(t, "b"), mustTask(t, "b"), mustTask(t, "c")}
	for _, tc := range []struct {
		index int
		raw   string
		want  int
	}{
		{0, "a", 0},  // where it was
		{2, "a", 0},  // moved: the one task with its line
		{9, "c", 3},  // past the end of the list now
		{1, "b", 1},  // two tasks with its line, one where it was
		{2, "b", 2},  //
		{0, "b", -1}, // two, neither where it was: can't say which
		{0, "gone", -1},
		{-1, "c", 3}, // no number: by its line alone
	} {
		if got := domain.FindTask(tasks, tc.index, tc.raw); got != tc.want {
			t.Errorf("FindTask(%d, %q) = %d, want %d", tc.index, tc.raw, got, tc.want)
		}
	}
}

// The review's case: `htt todo edit 2` opens charlie; while the editor
// is open, another htt completes task 0. The edit replaces charlie, now
// task 1, not delta, now task 2.
func TestReplaceFound_FollowsTheTaskByItsLine(t *testing.T) {
	repo := newRepo(t)
	seed(t, repo, "todo", "alpha", "bravo", "charlie", "delta pay the rent due:2026-12-01", "echo")
	ctx := loadCtx(t, repo, "todo")
	raw := ctx.Tasks[2].Raw
	if _, err := loadCtx(t, repo, "todo").Complete("0", domain.SubtasksAsk); err != nil { // elsewhere
		t.Fatal(err)
	}
	before, err := ctx.ReplaceFound(2, raw, mustTask(t, "charlie EDITED"))
	if err != nil {
		t.Fatal(err)
	}
	if before.Raw != "charlie" {
		t.Errorf("replaced %q, want charlie", before.Raw)
	}
	wantLines(t, "todo", rawsOf(t, repo, "todo"), "bravo", "charlie EDITED", "delta pay the rent due:2026-12-01", "echo")
}

// A task changed, gone or no longer told from another meanwhile is
// left alone: ErrTaskChanged, nothing saved.
func TestReplaceFound_ATaskChangedMeanwhileIsLeftAlone(t *testing.T) {
	for name, elsewhere := range map[string]func(ctx *domain.Context) error{
		"changed": func(ctx *domain.Context) error {
			_, err := ctx.Replace("2", mustTask(t, "charlie changed"))
			return err
		},
		"gone": func(ctx *domain.Context) error { _, err := ctx.Delete("2"); return err },
		"doubled, and moved": func(ctx *domain.Context) error {
			if _, err := ctx.Replace("0", mustTask(t, "charlie")); err != nil {
				return err
			}
			_, err := ctx.Delete("1") // charlie, charlie, delta: neither where it was
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			repo := newRepo(t)
			seed(t, repo, "todo", "alpha", "bravo", "charlie", "delta")
			ctx := loadCtx(t, repo, "todo")
			if err := elsewhere(loadCtx(t, repo, "todo")); err != nil {
				t.Fatal(err)
			}
			want := rawsOf(t, repo, "todo")
			if _, err := ctx.ReplaceFound(2, "charlie", mustTask(t, "charlie EDITED")); !errors.Is(err, domain.ErrTaskChanged) {
				t.Errorf("err = %v, want ErrTaskChanged", err)
			}
			wantLines(t, "todo", rawsOf(t, repo, "todo"), want...)
		})
	}
}

// The answer to the question about open subtasks completes the task
// asked about, wherever it is now, with the subtasks counted; when they
// changed, nothing is completed.
func TestCompleteFound(t *testing.T) {
	lines := []string{"alpha", "bravo id:1", "bravo-sub p:1", "charlie id:2", "charlie-sub-one p:2", "charlie-sub-two p:2", "delta"}

	repo := newRepo(t)
	seed(t, repo, "todo", lines...)
	ctx := loadCtx(t, repo, "todo")
	if _, err := loadCtx(t, repo, "todo").Complete("0", domain.SubtasksAsk); err != nil { // elsewhere
		t.Fatal(err)
	}
	done, err := ctx.CompleteFound(1, "bravo id:1", []string{"bravo-sub p:1"}, domain.SubtasksComplete)
	if err != nil {
		t.Fatal(err)
	}
	wantLines(t, "completed", rawsOfTasks(done), "x "+today+" bravo id:1 context:todo", "x "+today+" bravo-sub p:1 context:todo")
	wantLines(t, "todo", rawsOf(t, repo, "todo"), lines[3:]...)

	for name, tc := range map[string]struct {
		elsewhere func(ctx *domain.Context) error
		want      error
	}{
		"a subtask added": {func(ctx *domain.Context) error { return ctx.AddSubtask("1", mustTask(t, "2026-01-01 another")) }, domain.ErrSubtasksChanged},
		"a subtask done":  {func(ctx *domain.Context) error { _, err := ctx.Complete("2", domain.SubtasksAsk); return err }, domain.ErrSubtasksChanged},
		"the task changed": {func(ctx *domain.Context) error {
			_, _, err := ctx.SetPriority("1", "A")
			return err
		}, domain.ErrTaskChanged},
	} {
		t.Run(name, func(t *testing.T) {
			repo := newRepo(t)
			seed(t, repo, "todo", lines...)
			ctx := loadCtx(t, repo, "todo")
			if err := tc.elsewhere(loadCtx(t, repo, "todo")); err != nil {
				t.Fatal(err)
			}
			want := rawsOf(t, repo, "todo")
			wantDone := rawsOf(t, repo, domain.DoneContextName)
			if _, err := ctx.CompleteFound(1, "bravo id:1", []string{"bravo-sub p:1"}, domain.SubtasksComplete); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
			wantLines(t, "todo", rawsOf(t, repo, "todo"), want...)
			wantLines(t, "done", rawsOf(t, repo, domain.DoneContextName), wantDone...)
		})
	}
}
