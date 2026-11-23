package storage

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bricef/htt/internal/domain"
)

// The domain's operations over files, as transactions
// (docs/design/safe-writes.md): a write by another program between the
// read and the check runs the operation again, and an operation that
// changes several files writes the one gaining tasks first.

func fileRepo(t *testing.T, files map[string]string) (*FileRepository, string) {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		writeFile(t, filepath.Join(dir, name), content)
	}
	return NewFileRepository(dir, dir), dir
}

func loaded(t *testing.T, r domain.Repository, name string) *domain.Context {
	t.Helper()
	ctx, err := r.Context(name)
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

// An editor saving the file between the operation's read and its check:
// the operation runs again on the editor's version, and both changes
// are kept.
func TestFileTransact_AnEditorsSaveUnderneathIsKept(t *testing.T) {
	r, dir := fileRepo(t, map[string]string{"todo.txt": "finish me\nstay\n"})
	todo := filepath.Join(dir, "todo.txt")
	ran := 0
	r.hooks.underneath = func(run int) {
		ran = run
		if run == 1 {
			writeFile(t, todo, "finish me\nstay\nadded in an editor\n")
		}
	}
	ctx := loaded(t, r, "todo")
	if _, err := ctx.Complete("0", domain.SubtasksAsk); err != nil {
		t.Fatal(err)
	}
	if ran != 2 {
		t.Errorf("ran %d times, want 2", ran)
	}
	if got := readString(t, todo); got != "stay\nadded in an editor\n" {
		t.Errorf("todo.txt = %q", got)
	}
	if got := readString(t, filepath.Join(dir, "done.txt")); !strings.Contains(got, " finish me ") || strings.Count(got, "\n") != 1 {
		t.Errorf("done.txt = %q, want the task, once", got)
	}
	if got := rawLines(ctx.Tasks); !slices.Equal(got, []string{"stay", "added in an editor"}) {
		t.Errorf("the caller's context: %q", got)
	}
}

// A file changing under every run: nothing is written, and the error
// names the file.
func TestFileTransact_AFileThatKeepsChangingSavesNothing(t *testing.T) {
	r, dir := fileRepo(t, map[string]string{"todo.txt": "finish me\n"})
	todo := filepath.Join(dir, "todo.txt")
	r.hooks.underneath = func(run int) { writeFile(t, todo, fmt.Sprintf("finish me\nedit %d\n", run)) }
	_, err := loaded(t, r, "todo").Complete("0", domain.SubtasksAsk)
	if !errors.Is(err, domain.ErrKeepsChanging) || !strings.HasPrefix(err.Error(), todo+" keeps changing") {
		t.Fatalf("err = %v, want %s keeps changing", err, todo)
	}
	if got := readString(t, todo); got != "finish me\nedit 5\n" {
		t.Errorf("todo.txt = %q, want the editor's last version", got)
	}
	if left := leftovers(t, dir, "todo.txt"); len(left) > 0 {
		t.Errorf("written: %v", left)
	}
}

// Completing, deleting and moving write the file gaining the task
// first; when the write of the file losing it fails, the task is in
// both, never in neither.
func TestFileTransact_TheGainingFileIsWrittenFirst(t *testing.T) {
	ops := []struct {
		name    string
		gaining string
		op      func(ctx *domain.Context) error
	}{
		{"complete", "done.txt", func(ctx *domain.Context) error {
			_, err := ctx.Complete("0", domain.SubtasksAsk)
			return err
		}},
		{"delete", "archive.txt", func(ctx *domain.Context) error {
			_, err := ctx.Delete("0")
			return err
		}},
		{"move", "work.txt", func(ctx *domain.Context) error {
			_, err := ctx.Move("0", "work")
			return err
		}},
	}
	for _, op := range ops {
		t.Run(op.name, func(t *testing.T) {
			r, dir := fileRepo(t, map[string]string{"todo.txt": "the task\nstay\n"})
			var order []string
			r.hooks.replacing = func(path string) error {
				order = append(order, filepath.Base(path))
				if filepath.Base(path) == "todo.txt" {
					return errors.New("no space left on device")
				}
				return nil
			}
			if err := op.op(loaded(t, r, "todo")); err == nil || !strings.Contains(err.Error(), "no space") {
				t.Fatalf("err = %v, want the failed write", err)
			}
			if !slices.Equal(order, []string{op.gaining, "todo.txt"}) {
				t.Errorf("written in the order %v, want %s first", order, op.gaining)
			}
			if got := readString(t, filepath.Join(dir, "todo.txt")); got != "the task\nstay\n" {
				t.Errorf("todo.txt = %q, want the task still there", got)
			}
			if got := readString(t, filepath.Join(dir, op.gaining)); !strings.Contains(got, "the task") {
				t.Errorf("%s = %q, want the task there too", op.gaining, got)
			}
		})
	}
}

// The transaction's view reads what the change saved: a context saved,
// then loaded again in the same transaction, is the one saved, and a
// new context is listed.
func TestFileTransact_ReadsWhatItSaved(t *testing.T) {
	r, _ := fileRepo(t, map[string]string{"todo.txt": "one\n"})
	err := r.Transact(func(tx domain.Repository) error {
		ctx := loaded(t, tx, "new")
		ctx.Tasks = []*domain.Task{mustTask(t, "first")}
		if err := tx.Save(ctx); err != nil {
			return err
		}
		if got := rawLines(loaded(t, tx, "new").Tasks); !slices.Equal(got, []string{"first"}) {
			t.Errorf("loaded again: %q", got)
		}
		if names, _ := tx.ContextNames(); !slices.Equal(names, []string{"new", "todo"}) {
			t.Errorf("ContextNames in the transaction = %v", names)
		}
		return tx.SetCurrent("new")
	})
	if err != nil {
		t.Fatal(err)
	}
	if name, _ := r.CurrentContextName(); name != "new" {
		t.Errorf("current = %q", name)
	}
}

// A context kept from a transaction can't save into it once it's over:
// its operations run transactions of their own.
func TestFileTransact_AContextKeptFromATransactionRunsItsOwn(t *testing.T) {
	r, _ := fileRepo(t, map[string]string{"todo.txt": "one\n"})
	var kept *domain.Context
	if err := r.Transact(func(tx domain.Repository) error {
		kept = loaded(t, tx, "todo")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := kept.AddTask(mustTask(t, "2026-01-01 two")); err != nil {
		t.Fatalf("AddTask on a kept context: %v", err)
	}
	if got := rawLines(loaded(t, r, "todo").Tasks); !slices.Equal(got, []string{"one", "2026-01-01 two"}) {
		t.Errorf("todo = %q", got)
	}
}

// An editor saving the day's log between Append's read and its check:
// Append runs again, and both entries are kept.
func TestFileTransact_AnEditorsSaveOfTheLogIsKept(t *testing.T) {
	dir := t.TempDir()
	logs := NewFileTimelogRepository(dir)
	now := time.Now()
	path := logs.LogPath(now)
	day, err := logs.Day(now)
	if err != nil {
		t.Fatal(err)
	}
	ran := 0
	logs.hooks.underneath = func(run int) {
		ran = run
		if run == 1 {
			writeFile(t, path, "edited in ts:2026-01-01T00:00:00Z\n")
		}
	}
	if err := day.Append(mustTask(t, "mine")); err != nil {
		t.Fatal(err)
	}
	if ran != 2 {
		t.Errorf("ran %d times, want 2", ran)
	}
	got := readString(t, path)
	if !strings.HasPrefix(got, "edited in ts:2026-01-01T00:00:00Z\nmine ts:") || strings.Count(got, "\n") != 2 {
		t.Errorf("the log = %q, want both entries", got)
	}
	if bak := readString(t, path+".bak"); bak != "edited in ts:2026-01-01T00:00:00Z\n" {
		t.Errorf("the .bak = %q, want the edited version", bak)
	}
}

// Goroutines each with a repository of its own, as separate htt
// processes are, adding to and completing from one context at once:
// every change lands.
func TestFileTransact_ConcurrentChangesAllLand(t *testing.T) {
	// Six goroutines taking turns at 96 transactions, each parsing
	// both files, can wait longer than the usual bound under the race
	// detector; this test is about what lands, not how long it waits.
	shortLockWait(t, time.Minute)
	dir := t.TempDir()
	const workers, each = 6, 8
	var seed []string
	for i := range workers * each {
		seed = append(seed, fmt.Sprintf("finish %03d", i))
	}
	writeFile(t, filepath.Join(dir, "todo.txt"), strings.Join(seed, "\n")+"\n")

	errs := make(chan error, workers)
	for w := range workers {
		go func() {
			r := NewFileRepository(dir, dir)
			for i := range each {
				ctx, err := r.Context("todo")
				if err == nil {
					err = ctx.AddTask(mustTask(t, fmt.Sprintf("2026-01-01 new %d-%d", w, i)))
				}
				if err == nil {
					// Task 0 is a "finish" task: new ones go after them.
					_, err = ctx.Complete("0", domain.SubtasksAsk)
				}
				if err != nil {
					errs <- err
					return
				}
			}
			errs <- nil
		}()
	}
	for range workers {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}

	todo := rawLines(loaded(t, NewFileRepository(dir, dir), "todo").Tasks)
	done := rawLines(loaded(t, NewFileRepository(dir, dir), domain.DoneContextName).Tasks)
	if len(todo) != workers*each || len(done) != workers*each {
		t.Errorf("todo has %d tasks and done %d, want %d each", len(todo), len(done), workers*each)
	}
	for w := range workers {
		for i := range each {
			name := fmt.Sprintf(" new %d-%d", w, i)
			if !slices.ContainsFunc(todo, func(l string) bool { return strings.HasSuffix(l, name) }) {
				t.Errorf("%q is lost", name)
			}
		}
	}
	for _, l := range todo {
		if strings.Contains(l, "finish") {
			t.Errorf("%q wasn't completed", l)
		}
	}
}

func rawLines(tasks []*domain.Task) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.Raw
	}
	return out
}
