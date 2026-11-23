package e2e

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

// Safe writes (docs/design/safe-writes.md): several htt processes
// changing the same files at once lose nothing, and a reader never sees
// a file missing or half-written.

// safeEnv is an env whose htt never checks for updates, for tests that
// run many htt processes at once.
func safeEnv(t *testing.T) *env {
	t.Helper()
	e := newEnv(t)
	e.writeConfig("check_for_updates: false\n")
	return e
}

func TestSafeWrites_AReaderNeverSeesAPartFile(t *testing.T) {
	e := safeEnv(t)
	const tasks = 400
	var seed []string
	for i := range tasks {
		seed = append(seed, fmt.Sprintf("task %03d with some text to make the file longer", i))
	}
	e.writeData("todo.txt", strings.Join(seed, "\n")+"\n")
	path := filepath.Join(e.dataDir, "todo.txt")

	// Each rewrite replaces the first task: "rewrite 00" and on.
	const rewrites = 30
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for i := range rewrites {
			r := e.run("todo", "replace", "0", fmt.Sprintf("rewrite %02d", i))
			if r.exitCode != 0 {
				t.Errorf("rewrite %d: exit %d: %s", i, r.exitCode, r.stderr)
				return
			}
		}
	}()

	var reads, missing, partial int
	var example string
	deadline := time.After(2 * time.Minute)
	for done := false; !done && reads < 1_000_000; reads++ {
		select {
		case <-finished:
			done = true
		case <-deadline:
			t.Fatal("the rewrites took over two minutes")
		default:
		}
		b, err := os.ReadFile(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			missing++
			continue
		case err != nil:
			t.Fatalf("read %s: %v", path, err)
		}
		if !wholeVersion(string(b), seed) {
			partial++
			if example == "" {
				example = string(b)
			}
		}
	}
	t.Logf("%d reads while htt rewrote the file %d times", reads, rewrites)
	if missing > 0 || partial > 0 {
		t.Errorf("of %d reads, %d found no file and %d part of one; the first part:\n%.300q", reads, missing, partial, example)
	}
}

// wholeVersion reports whether content is the seed file with its first
// line as written, or as one of the rewrites left it.
func wholeVersion(content string, seed []string) bool {
	lines := strings.Split(content, "\n")
	if len(lines) != len(seed)+1 || lines[len(seed)] != "" {
		return false
	}
	if lines[0] != seed[0] && !strings.HasPrefix(lines[0], "rewrite ") {
		return false
	}
	return slices.Equal(lines[1:len(seed)], seed[1:])
}

// holdLock holds htt's lock on dir as another htt in the middle of a
// change would, until release or the end of the test. htt keeps it in
// .htt.lock, in each directory it writes.
func holdLock(t *testing.T, dir string) (release func()) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	l := flock.New(filepath.Join(dir, ".htt.lock"))
	if ok, err := l.TryLock(); err != nil || !ok {
		t.Fatalf("take htt's lock: %v, %v", ok, err)
	}
	release = func() { _ = l.Unlock() }
	t.Cleanup(release)
	return release
}

// Another htt holding the lock: htt waits a few seconds, says another
// htt is busy, and changes nothing. Once the lock is let go of, the same
// command goes through.
func TestSafeWrites_ABusyLockSaysSoAndChangesNothing(t *testing.T) {
	e := safeEnv(t)
	e.mustRun("todo", "add", "before")
	before := e.readData("todo.txt")
	release := holdLock(t, e.dataDir)

	began := time.Now()
	r := e.run("todo", "add", "while busy")
	took := time.Since(began)
	if r.exitCode == 0 {
		t.Fatalf("htt todo add succeeded while another htt held the lock:\n%s", r.stdout)
	}
	assertContains(t, "stderr", r.stderr, "another htt is busy with your files; try again")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), before)
	if took < 2*time.Second || took > 30*time.Second {
		t.Errorf("htt gave up after %v; want after a few seconds", took)
	}

	release()
	e.mustRun("todo", "add", "once free")
	assertContains(t, "todo.txt", e.readData("todo.txt"), "once free")
}

// The lock file is no context: htt's lists never show it.
func TestSafeWrites_TheLockFileIsNoContext(t *testing.T) {
	e := safeEnv(t)
	e.mustRun("todo", "add", "a task")
	if !e.dataExists(".htt.lock") {
		t.Fatalf("no lock file in the data directory")
	}
	r := e.mustRun("todo", "status")
	if strings.Contains(r.stdout, "htt") || strings.Contains(r.stdout, "lock") {
		t.Errorf("todo status shows the lock file:\n%s", r.stdout)
	}
}
