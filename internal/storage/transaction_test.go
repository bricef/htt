package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bricef/htt/internal/domain"
	"github.com/gofrs/flock"
)

// The mechanism (transaction.go, docs/design/safe-writes.md): the
// transaction, the lock, the check before replacing and the atomic
// replacement, through the file repositories and directly.

// shortLockWait makes a held lock give up after d, for the test.
func shortLockWait(t *testing.T, d time.Duration) {
	t.Helper()
	was := lockWait
	lockWait = d
	t.Cleanup(func() { lockWait = was })
}

// holdLock holds htt's lock on dir, as another htt in a transaction
// would, until the test ends or release is called.
func holdLock(t *testing.T, dir string) (release func()) {
	t.Helper()
	if err := os.MkdirAll(dir, dirMode); err != nil {
		t.Fatal(err)
	}
	l := flock.New(filepath.Join(dir, lockName))
	ok, err := l.TryLock()
	if err != nil || !ok {
		t.Fatalf("take the lock: %v, %v", ok, err)
	}
	var once sync.Once
	release = func() { once.Do(func() { _ = l.Unlock() }) }
	t.Cleanup(release)
	return release
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), fileMode); err != nil {
		t.Fatal(err)
	}
}

// readAsAReader reads path as another program would while htt writes
// it. On Windows a file can't be opened at the moment one is renamed
// over it (a sharing violation), which a reader there meets and tries
// again, as htt's own reads do (readFile); and a reader with the file
// open in a tight loop would keep htt's rename waiting, so there it
// pauses a millisecond before each read.
func readAsAReader(path string) ([]byte, error) {
	if runtime.GOOS != "windows" {
		return os.ReadFile(path)
	}
	time.Sleep(time.Millisecond)
	b, err := os.ReadFile(path)
	for tries := 0; err != nil && heldOpen(err) && tries < 200; tries++ {
		time.Sleep(time.Millisecond)
		b, err = os.ReadFile(path)
	}
	return b, err
}

func readString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// leftovers lists what in dir isn't one of names, the lock file or a
// .bak: a temporary file left behind, say.
func leftovers(t *testing.T, dir string, names ...string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if n == lockName || strings.HasSuffix(n, ".bak") || slices.Contains(names, n) {
			continue
		}
		out = append(out, n)
	}
	return out
}

// tasks is a context of n numbered tasks, each line long enough that a
// file of them is written in more than one piece.
func tasks(name string, n int, tag string) *domain.Context {
	ctx := &domain.Context{Name: name}
	for i := range n {
		task, err := domain.NewTask(fmt.Sprintf("task %04d %s %s", i, tag, strings.Repeat("words ", 10)))
		if err != nil {
			panic(err)
		}
		ctx.Tasks = append(ctx.Tasks, task)
	}
	return ctx
}

// While one goroutine saves a context again and again, another reading
// its file sees each time a whole version: never no file, never part of
// one.
func TestTransact_AReaderNeverSeesAPartFile(t *testing.T) {
	dir := t.TempDir()
	r := NewFileRepository(dir, dir)
	versions := []*domain.Context{tasks("todo", 500, "one"), tasks("todo", 500, "two")}
	whole := map[string]bool{}
	for _, v := range versions {
		whole[string(contextFile(v))] = true
	}
	if err := r.Save(versions[0]); err != nil {
		t.Fatal(err)
	}

	const saves = 200
	finished := make(chan error, 1)
	go func() {
		for i := range saves {
			if err := r.Save(versions[i%2]); err != nil {
				finished <- err
				return
			}
		}
		finished <- nil
	}()

	path := r.ContextPath("todo")
	var reads, missing, partial int
	deadline := time.After(time.Minute)
	for done := false; !done; reads++ {
		select {
		case err := <-finished:
			if err != nil {
				t.Fatalf("save: %v", err)
			}
			done = true
		case <-deadline:
			t.Fatal("the saves took over a minute")
		default:
		}
		b, err := readAsAReader(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			missing++
		case err != nil:
			t.Fatalf("read: %v", err)
		case !whole[string(b)]:
			partial++
		}
	}
	t.Logf("%d reads while saving %d times", reads, saves)
	if missing > 0 || partial > 0 {
		t.Errorf("of %d reads while saving, %d found no file and %d part of one", reads, missing, partial)
	}
	if left := leftovers(t, dir, "todo.txt"); len(left) > 0 {
		t.Errorf("left behind: %v", left)
	}
}

// Saving keeps the previous version as .bak, for contexts and logs; the
// pointer has none.
func TestTransact_KeepsThePreviousVersionAsBak(t *testing.T) {
	dir := t.TempDir()
	r := NewFileRepository(dir, dir)
	for _, raw := range []string{"first", "second", "third"} {
		if err := r.Save(&domain.Context{Name: "todo", Tasks: []*domain.Task{mustTask(t, raw)}}); err != nil {
			t.Fatal(err)
		}
	}
	if got := readString(t, filepath.Join(dir, "todo.txt.bak")); got != "second\n" {
		t.Errorf("todo.txt.bak = %q, want the version before the last save", got)
	}
	if err := r.SetCurrent("work"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetCurrent("home"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, currentContextFile+".bak")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the pointer got a .bak: %v", err)
	}
	if left := leftovers(t, dir, "todo.txt", currentContextFile); len(left) > 0 {
		t.Errorf("left behind: %v", left)
	}
}

// Another htt holding the lock: a save waits for it, then gives up with
// ErrBusy, changing nothing.
func TestTransact_AHeldLockGivesErrBusy(t *testing.T) {
	shortLockWait(t, 200*time.Millisecond)
	dir := t.TempDir()
	r := NewFileRepository(dir, dir)
	writeFile(t, r.ContextPath("todo"), "before\n")
	holdLock(t, dir)

	began := time.Now()
	err := r.Save(&domain.Context{Name: "todo", Tasks: []*domain.Task{mustTask(t, "after")}})
	took := time.Since(began)
	if !errors.Is(err, domain.ErrBusy) {
		t.Fatalf("Save with the lock held = %v, want ErrBusy", err)
	}
	if err.Error() != "another htt is busy with your files; try again" {
		t.Errorf("the message: %q", err)
	}
	if took < 200*time.Millisecond || took > 5*time.Second {
		t.Errorf("gave up after %v; want after the wait, 200ms", took)
	}
	if got := readString(t, r.ContextPath("todo")); got != "before\n" {
		t.Errorf("todo.txt = %q, want it untouched", got)
	}
	if err := r.SetCurrent("work"); !errors.Is(err, domain.ErrBusy) {
		t.Errorf("SetCurrent with the lock held = %v, want ErrBusy", err)
	}
}

// A lock let go of within the wait: the save waits, then goes ahead.
func TestTransact_WaitsForTheLock(t *testing.T) {
	shortLockWait(t, 5*time.Second)
	dir := t.TempDir()
	r := NewFileRepository(dir, dir)
	release := holdLock(t, dir)
	time.AfterFunc(100*time.Millisecond, release)

	if err := r.Save(&domain.Context{Name: "todo", Tasks: []*domain.Task{mustTask(t, "after")}}); err != nil {
		t.Fatalf("Save once the lock was let go: %v", err)
	}
	if got := readString(t, r.ContextPath("todo")); got != "after\n" {
		t.Errorf("todo.txt = %q", got)
	}
}

// The time log's directory has its own lock: a transaction on the
// contexts doesn't hold it.
func TestTransact_LocksTheDirectoriesItWrites(t *testing.T) {
	shortLockWait(t, 100*time.Millisecond)
	data := t.TempDir()
	pointer := t.TempDir()
	logs := NewFileTimelogRepository(data)
	holdLock(t, logs.logDir())
	if err := NewFileRepository(data, pointer).Save(&domain.Context{Name: "todo"}); err != nil {
		t.Errorf("a context saved with the log directory locked: %v", err)
	}
	if err := logs.Save(domain.NewTimelog(nil, time.Now())); !errors.Is(err, domain.ErrBusy) {
		t.Errorf("a log saved with its directory locked: %v, want ErrBusy", err)
	}
	// A repository whose pointer is elsewhere locks there too.
	holdLock(t, pointer)
	if err := NewFileRepository(data, pointer).Save(&domain.Context{Name: "todo"}); !errors.Is(err, domain.ErrBusy) {
		t.Errorf("a context saved with the pointer's directory locked: %v, want ErrBusy", err)
	}
	if names, _ := NewFileRepository(data, pointer).ContextNames(); !slices.Equal(names, []string{"todo"}) {
		t.Errorf("ContextNames = %v: the lock file isn't a context", names)
	}
}

// Two names for one directory are one lock, not a transaction waiting
// on itself.
func TestTransact_OneDirectoryTwoNamesIsOneLock(t *testing.T) {
	shortLockWait(t, 200*time.Millisecond)
	dir := t.TempDir()
	r := NewFileRepository(dir, dir+string(filepath.Separator))
	if err := r.Save(&domain.Context{Name: "todo"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

// A change made underneath, by another program, between the read and
// the check: the change runs again on the new content and both changes
// are kept.
func TestTransact_AChangeUnderneathRunsTheChangeAgain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	writeFile(t, path, "one\n")
	h := hooks{underneath: func(run int) {
		if run == 1 {
			writeFile(t, path, "one\ntwo\n") // an editor's save
		}
	}}
	ran := 0
	err := transact([]string{dir}, h, func(tx *transaction) error {
		ran++
		data, _, err := tx.Read(path)
		if err != nil {
			return err
		}
		tx.Write(path, append(data, "three\n"...), true)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if ran != 2 {
		t.Errorf("the change ran %d times, want 2", ran)
	}
	if got := readString(t, path); got != "one\ntwo\nthree\n" {
		t.Errorf("todo.txt = %q, want both changes", got)
	}
	if got := readString(t, path+".bak"); got != "one\ntwo\n" {
		t.Errorf("todo.txt.bak = %q, want the version the change ran on", got)
	}
}

// A file that changes under every run: the transaction gives up,
// naming it, and writes nothing.
func TestTransact_AFileThatKeepsChangingGivesUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	other := filepath.Join(dir, "done.txt")
	writeFile(t, path, "one\n")
	h := hooks{underneath: func(run int) {
		writeFile(t, path, fmt.Sprintf("changed %d\n", run))
	}}
	ran := 0
	err := transact([]string{dir}, h, func(tx *transaction) error {
		ran++
		data, _, err := tx.Read(path)
		if err != nil {
			return err
		}
		tx.Write(other, data, true)
		tx.Write(path, nil, true)
		return nil
	})
	if !errors.Is(err, domain.ErrKeepsChanging) {
		t.Fatalf("err = %v, want ErrKeepsChanging", err)
	}
	if want := path + " keeps changing; nothing was saved"; err.Error() != want {
		t.Errorf("the message: %q, want %q", err, want)
	}
	if ran != runs {
		t.Errorf("the change ran %d times, want %d", ran, runs)
	}
	if got := readString(t, path); got != fmt.Sprintf("changed %d\n", runs) {
		t.Errorf("todo.txt = %q, want the other program's last change", got)
	}
	if _, err := os.Stat(other); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("done.txt was written: %v", err)
	}
	if left := leftovers(t, dir, "todo.txt"); len(left) > 0 {
		t.Errorf("left behind: %v", left)
	}
}

// A file that didn't exist when read and does by the check has changed.
func TestTransact_AFileCreatedUnderneathIsAChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	h := hooks{underneath: func(run int) {
		if run == 1 {
			writeFile(t, path, "made elsewhere\n")
		}
	}}
	err := transact([]string{dir}, h, func(tx *transaction) error {
		data, _, err := tx.Read(path)
		if err != nil {
			return err
		}
		tx.Write(path, append(data, "mine\n"...), true)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := readString(t, path); got != "made elsewhere\nmine\n" {
		t.Errorf("todo.txt = %q, want both", got)
	}
}

// The files are replaced in the order the change wrote them; a failure
// replacing the second leaves the first replaced and the second as it
// was, with no temporary file left behind.
func TestTransact_WritesInOrderAndStopsAtAFailure(t *testing.T) {
	dir := t.TempDir()
	gaining := filepath.Join(dir, "done.txt")
	losing := filepath.Join(dir, "todo.txt")
	writeFile(t, losing, "task\n")
	var order []string
	h := hooks{replacing: func(path string) error {
		order = append(order, filepath.Base(path))
		if path == losing {
			return errors.New("disk full")
		}
		return nil
	}}
	err := transact([]string{dir}, h, func(tx *transaction) error {
		tx.Write(gaining, []byte("x task\n"), true)
		tx.Write(losing, nil, true)
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("err = %v, want the failure", err)
	}
	if !slices.Equal(order, []string{"done.txt", "todo.txt"}) {
		t.Errorf("written in the order %v", order)
	}
	if got := readString(t, gaining); got != "x task\n" {
		t.Errorf("done.txt = %q", got)
	}
	if got := readString(t, losing); got != "task\n" {
		t.Errorf("todo.txt = %q, want it as it was", got)
	}
	if left := leftovers(t, dir, "todo.txt", "done.txt"); len(left) > 0 {
		t.Errorf("left behind: %v", left)
	}
}

// appendTo appends s to path in place, as `echo ... >> todo.txt` does:
// another program that doesn't replace the file, but writes into it.
func appendTo(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// completeFirst is a change that moves todo's first line to done, as a
// completion does: done gains it, written first, and todo loses it.
func completeFirst(todo, done string) func(tx *transaction) error {
	return func(tx *transaction) error {
		data, _, err := tx.Read(todo)
		if err != nil {
			return err
		}
		first, rest, _ := strings.Cut(string(data), "\n")
		was, _, err := tx.Read(done)
		if err != nil {
			return err
		}
		tx.Write(done, append(slices.Clone(was), "x "+first+"\n"...), true)
		tx.Write(todo, []byte(rest), true)
		return nil
	}
}

// Another program appending to a file in place while htt writes its
// temporary files and syncs them (the slow part of a save, tens of
// milliseconds for a completion): the check comes after them, so it
// sees the line, and the change runs again keeping it. When the
// temporary files were written after the check, the rename discarded
// the line.
func TestTransact_AnAppendWhileTheFilesAreWrittenIsKept(t *testing.T) {
	dir := t.TempDir()
	todo, done := filepath.Join(dir, "todo.txt"), filepath.Join(dir, "done.txt")
	writeFile(t, todo, "finish me\nstay\n")
	appended := false
	h := hooks{writing: func(path string) error {
		if path == done && !appended {
			appended = true
			appendTo(t, todo, "appended elsewhere\n")
		}
		return nil
	}}
	ran := 0
	change := completeFirst(todo, done)
	err := transact([]string{dir}, h, func(tx *transaction) error {
		ran++
		return change(tx)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !appended {
		t.Fatal("the append never happened")
	}
	if got := readString(t, todo); got != "stay\nappended elsewhere\n" {
		t.Errorf("todo.txt = %q, want the appended line kept", got)
	}
	if got := readString(t, done); got != "x finish me\n" {
		t.Errorf("done.txt = %q", got)
	}
	if ran != 2 {
		t.Errorf("the change ran %d times, want 2", ran)
	}
	if left := leftovers(t, dir, "todo.txt", "done.txt"); len(left) > 0 {
		t.Errorf("left behind: %v", left)
	}
}

// Between the check and the renames nothing is written: every
// temporary file, the .bak ones included, is written and synced before
// the check, and only renames follow it, so another program's write
// can slip in unseen only in the moment the renames take.
func TestTransact_OnlyRenamesFollowTheCheck(t *testing.T) {
	dir := t.TempDir()
	todo, done := filepath.Join(dir, "todo.txt"), filepath.Join(dir, "done.txt")
	writeFile(t, todo, "finish me\nstay\n")
	writeFile(t, done, "x old\n")
	var events []string
	h := hooks{
		writing: func(path string) error {
			events = append(events, "write "+filepath.Base(path))
			return nil
		},
		underneath: func(int) { events = append(events, "check") },
		replacing: func(path string) error {
			events = append(events, "rename "+filepath.Base(path))
			return nil
		},
	}
	if err := transact([]string{dir}, h, completeFirst(todo, done)); err != nil {
		t.Fatal(err)
	}
	check := slices.Index(events, "check")
	writes, renames := 0, 0
	for i, e := range events {
		switch {
		case strings.HasPrefix(e, "write "):
			writes++
			if i > check {
				t.Errorf("%s after the check", e)
			}
		case strings.HasPrefix(e, "rename "):
			renames++
			if i < check {
				t.Errorf("%s before the check", e)
			}
		}
	}
	// done.txt, todo.txt and the .bak of each.
	if writes != 4 || renames != 2 || check < 0 {
		t.Errorf("events %q: want four files written, the check, then two files replaced", events)
	}
	if got := readString(t, done+".bak"); got != "x old\n" {
		t.Errorf("done.txt.bak = %q", got)
	}
	if got := readString(t, todo+".bak"); got != "finish me\nstay\n" {
		t.Errorf("todo.txt.bak = %q", got)
	}
}

// A failure writing a temporary file, a full disk say, writes nothing:
// no file is replaced, since none is until every one is written, and no
// temporary file is left behind.
func TestTransact_AFailureWritingTheFilesReplacesNone(t *testing.T) {
	dir := t.TempDir()
	todo, done := filepath.Join(dir, "todo.txt"), filepath.Join(dir, "done.txt")
	writeFile(t, todo, "finish me\nstay\n")
	h := hooks{writing: func(path string) error {
		if path == todo {
			return errors.New("no space left on device")
		}
		return nil
	}}
	err := transact([]string{dir}, h, completeFirst(todo, done))
	if err == nil || !strings.Contains(err.Error(), "no space") {
		t.Fatalf("err = %v, want the failure", err)
	}
	if got := readString(t, todo); got != "finish me\nstay\n" {
		t.Errorf("todo.txt = %q, want it as it was", got)
	}
	if _, err := os.Stat(done); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("done.txt was written: %v", err)
	}
	if left := leftovers(t, dir, "todo.txt"); len(left) > 0 {
		t.Errorf("left behind: %v", left)
	}
}

// An error from the change writes nothing, and comes back as it is.
func TestTransact_AnErrorFromTheChangeWritesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	writeFile(t, path, "before\n")
	refused := errors.New("refused")
	err := transact([]string{dir}, hooks{}, func(tx *transaction) error {
		tx.Write(path, []byte("after\n"), true)
		return refused
	})
	if err != refused {
		t.Fatalf("err = %v, want the change's", err)
	}
	if got := readString(t, path); got != "before\n" {
		t.Errorf("todo.txt = %q", got)
	}
}

// A transaction writes only in the directories it locked.
func TestTransact_RefusesToWriteOutsideItsDirectories(t *testing.T) {
	dir, elsewhere := t.TempDir(), t.TempDir()
	err := transact([]string{dir}, hooks{}, func(tx *transaction) error {
		tx.Write(filepath.Join(elsewhere, "todo.txt"), []byte("x\n"), true)
		return nil
	})
	if err == nil {
		t.Fatal("wrote outside its directories")
	}
	if left := leftovers(t, elsewhere); len(left) > 0 {
		t.Errorf("wrote %v", left)
	}
}

// A file another process has open is still replaced: on Windows, which
// refuses to rename over an open file, once it's closed.
func TestReplace_AFileHeldOpenIsReplacedOnceClosed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	writeFile(t, path, "before\n")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	time.AfterFunc(200*time.Millisecond, func() { _ = f.Close() })
	staged, err := stage(path, []byte("after\n"))
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	if err := staged.replace(); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := readString(t, path); got != "after\n" {
		t.Errorf("todo.txt = %q", got)
	}
	if left := leftovers(t, dir, "todo.txt"); len(left) > 0 {
		t.Errorf("left behind: %v", left)
	}
}
