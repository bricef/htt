package storage

import (
	"errors"
	"fmt"
	"os"
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

// Completing, deleting and moving to the done file append to the done
// file or the archive: their lines stay as they were, byte for byte,
// and the task's line goes after them, on a line of its own. They
// aren't parsed and written back, which held the lock for seconds once
// the done file held a few thousand tasks.
func TestFileTransact_TheDoneFileAndTheArchiveAreAppendedTo(t *testing.T) {
	const kept = "x 2026-01-01 old one\n\n   x 2026-01-02 indented, with a blank line above\nx 2026-01-03 no newline at the end"
	for _, op := range []struct {
		name, file string
		run        func(ctx *domain.Context) error
		want       string
	}{
		{"complete", "done.txt", func(ctx *domain.Context) error {
			_, err := ctx.Complete("0", domain.SubtasksAsk)
			return err
		}, " the task context:todo\n"},
		{"delete", "archive.txt", func(ctx *domain.Context) error {
			_, err := ctx.Delete("0")
			return err
		}, "the task archived-from:todo deleted-on:"},
		{"move to done", "done.txt", func(ctx *domain.Context) error {
			_, err := ctx.Move("0", domain.DoneContextName)
			return err
		}, "the task\n"},
	} {
		t.Run(op.name, func(t *testing.T) {
			r, dir := fileRepo(t, map[string]string{"todo.txt": "the task\nstay\n", op.file: kept})
			if err := op.run(loaded(t, r, "todo")); err != nil {
				t.Fatal(err)
			}
			got := readString(t, filepath.Join(dir, op.file))
			added, ok := strings.CutPrefix(got, kept+"\n")
			if !ok || strings.Count(added, "\n") != 1 || !strings.Contains(added, op.want) {
				t.Errorf("%s = %q, want what it held, then the task's line (%q)", op.file, got, op.want)
			}
			if got := readString(t, filepath.Join(dir, op.file+".bak")); got != kept {
				t.Errorf("%s.bak = %q", op.file, got)
			}
			if got := readString(t, filepath.Join(dir, "todo.txt")); got != "stay\n" {
				t.Errorf("todo.txt = %q", got)
			}
		})
	}
}

// The files an operation leaves don't depend on when it read them: on
// a context loaded before another program changed the file (which the
// transaction rereads, parsing only the lines it didn't hold), or
// meeting the change underneath (which runs it again), an operation
// leaves the files as it does on a context loaded after the change.
// The review's differential, over every operation and change below.
func TestFileTransact_WhenTheContextWasReadDoesntMatter(t *testing.T) {
	seedFiles := map[string]string{
		"todo.txt":    "(A) Plan id:1\nWrite p:1\nBook p:1 id:2\nCall p:2\nPack\nZeta\n",
		"other.txt":   "Other one id:1\nOther sub p:1\n",
		"done.txt":    "x 2026-01-01 2025-12-01 old done context:todo\n",
		"archive.txt": "gone archived-from:todo deleted-on:2026-01-01\n",
	}
	task := func(s string) *domain.Task { return mustTask(t, s) }
	ops := map[string]func(ctx *domain.Context) error{
		"add":         func(c *domain.Context) error { return c.AddTask(task("new one")) },
		"add under 4": func(c *domain.Context) error { return c.AddSubtask("4", task("sub of four")) },
		"add under 0": func(c *domain.Context) error { return c.AddSubtask("0", task("sub of zero")) },
		"x 0, all":    func(c *domain.Context) error { _, err := c.Complete("0", domain.SubtasksComplete); return err },
		"x 0, alone":  func(c *domain.Context) error { _, err := c.Complete("0", domain.SubtasksLeave); return err },
		"x 4":         func(c *domain.Context) error { _, err := c.Complete("4", domain.SubtasksAsk); return err },
		"rm 2":        func(c *domain.Context) error { _, err := c.Delete("2"); return err },
		"rm 0":        func(c *domain.Context) error { _, err := c.Delete("0"); return err },
		"mv 0 other":  func(c *domain.Context) error { _, err := c.Move("0", "other"); return err },
		"mv 2 other":  func(c *domain.Context) error { _, err := c.Move("2", "other"); return err },
		"mv 4 done":   func(c *domain.Context) error { _, err := c.Move("4", "done"); return err },
		"replace 5":   func(c *domain.Context) error { _, err := c.Replace("5", task("Zeta changed")); return err },
		"up 4":        func(c *domain.Context) error { _, _, err := c.IncreasePriority("4"); return err },
		"down 0":      func(c *domain.Context) error { _, _, err := c.DecreasePriority("0"); return err },
		"pri 5 B":     func(c *domain.Context) error { _, _, err := c.SetPriority("5", "B"); return err },
		"nest 5 4":    func(c *domain.Context) error { _, _, err := c.Nest("5", "4"); return err },
		"unnest 3":    func(c *domain.Context) error { _, err := c.Unnest("3"); return err },
		"tab 5":       func(c *domain.Context) error { _, err := c.NestUnderPrevious("5"); return err },
		"shift-tab 3": func(c *domain.Context) error { _, err := c.UnnestOneLevel("3"); return err },
	}
	edit := func(dir, name string, f func(string) string) {
		p := filepath.Join(dir, name)
		b, _ := os.ReadFile(p)
		writeFile(t, p, f(string(b)))
	}
	changes := map[string]func(dir string){
		"append": func(dir string) { edit(dir, "todo.txt", func(s string) string { return s + "EXT appended\n" }) },
		"prepend": func(dir string) {
			edit(dir, "todo.txt", func(s string) string { return "EXT first\n" + s })
		},
		"the other files": func(dir string) {
			for _, n := range []string{"done.txt", "archive.txt", "other.txt"} {
				edit(dir, n, func(s string) string { return s + "EXT in " + n + " id:9\n" })
			}
		},
		"an id taken": func(dir string) {
			edit(dir, "todo.txt", func(s string) string { return s + "EXT holder id:3\nEXT child p:3\n" })
		},
		"the first removed": func(dir string) {
			edit(dir, "todo.txt", func(s string) string { _, rest, _ := strings.Cut(s, "\n"); return rest })
		},
		"a task changed": func(dir string) {
			edit(dir, "todo.txt", func(s string) string { return strings.Replace(s, "Pack\n", "(B) Pack +van\n", 1) })
		},
	}
	files := func(dir string) string {
		var b strings.Builder
		for _, n := range []string{"todo.txt", "other.txt", "done.txt", "archive.txt"} {
			got, _ := os.ReadFile(filepath.Join(dir, n))
			fmt.Fprintf(&b, "== %s\n%s", n, got)
		}
		return b.String()
	}
	for opName, op := range ops {
		for changeName, change := range changes {
			t.Run(opName+", "+changeName, func(t *testing.T) {
				// The reference: changed, then loaded, then the operation.
				ref, refDir := fileRepo(t, seedFiles)
				change(refDir)
				refErr := op(loaded(t, ref, "todo"))

				// Loaded, changed, then the operation.
				stale, staleDir := fileRepo(t, seedFiles)
				ctx := loaded(t, stale, "todo")
				change(staleDir)
				staleErr := op(ctx)

				// Loaded, then changed underneath its first run.
				under, underDir := fileRepo(t, seedFiles)
				under.hooks.underneath = func(run int) {
					if run == 1 {
						change(underDir)
					}
				}
				underCtx := loaded(t, under, "todo")
				underErr := op(underCtx)

				for _, got := range []struct {
					how string
					err error
					dir string
					r   *FileRepository
					ctx *domain.Context
				}{{"loaded before the change", staleErr, staleDir, stale, ctx}, {"changed underneath", underErr, underDir, under, underCtx}} {
					if fmt.Sprint(got.err) != fmt.Sprint(refErr) {
						t.Errorf("%s: err %v, want %v", got.how, got.err, refErr)
					}
					if a, b := files(got.dir), files(refDir); a != b {
						t.Errorf("%s: the files\n%s\nwant\n%s", got.how, a, b)
					}
					if got.err == nil {
						if a, b := rawLines(got.ctx.Tasks), rawLines(loaded(t, got.r, "todo").Tasks); !slices.Equal(a, b) {
							t.Errorf("%s: the caller's context %q, the file %q", got.how, a, b)
						}
					}
				}
			})
		}
	}
}

// A body composing operations (Context.Transact) can save the losing
// file first: adding a task saves todo, then completing one saves done
// and todo. The gaining file is written first all the same.
func TestFileTransact_AComposedBodyWritesTheGainingFileFirst(t *testing.T) {
	r, dir := fileRepo(t, map[string]string{"todo.txt": "the task\nstay\n"})
	var order []string
	r.hooks.replacing = func(path string) error {
		order = append(order, filepath.Base(path))
		if len(order) == 2 {
			return errors.New("no space left on device")
		}
		return nil
	}
	err := loaded(t, r, "todo").Transact(func(c *domain.Context) error {
		if err := c.AddTask(mustTask(t, "2026-01-01 new")); err != nil {
			return err
		}
		_, err := c.Complete("0", domain.SubtasksAsk)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "no space") {
		t.Fatalf("err = %v, want the failed write", err)
	}
	if !slices.Equal(order, []string{"done.txt", "todo.txt"}) {
		t.Errorf("written in the order %v, want done.txt first", order)
	}
	if got := readString(t, filepath.Join(dir, "done.txt")); !strings.Contains(got, "the task") {
		t.Errorf("done.txt = %q, want the task", got)
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
