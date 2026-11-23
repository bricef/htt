package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

// readAsAReader reads path as another program would while htt writes
// it. On Windows a file can't be opened at the moment one is renamed
// over it, which a reader there meets (access denied, or a sharing
// violation) and tries again, as htt's own reads do; and a reader with
// the file open in a tight loop would keep htt's rename waiting, so
// there it pauses a millisecond before each read.
func readAsAReader(path string) ([]byte, error) {
	if runtime.GOOS != "windows" {
		return os.ReadFile(path)
	}
	heldOpen := func(err error) bool {
		var errno syscall.Errno
		return errors.As(err, &errno) && (errno == 5 || errno == 32)
	}
	time.Sleep(time.Millisecond)
	b, err := os.ReadFile(path)
	for tries := 0; err != nil && heldOpen(err) && tries < 200; tries++ {
		time.Sleep(time.Millisecond)
		b, err = os.ReadFile(path)
	}
	return b, err
}

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

// started is an htt process started by start, with its output.
type started struct {
	args           []string
	cmd            *exec.Cmd
	stdout, stderr bytes.Buffer
}

// start starts htt with args, as run does, killed at ctx's deadline.
func (e *env) start(ctx context.Context, args ...string) *started {
	e.t.Helper()
	p := &started{args: args}
	p.cmd = exec.CommandContext(ctx, binaryPath, append([]string{"--no-color"}, args...)...)
	p.cmd.Env = append(append(e.homeEnv(), "PATH="+os.Getenv("PATH")), e.extraEnv...)
	p.cmd.Stdout, p.cmd.Stderr = &p.stdout, &p.stderr
	if err := p.cmd.Start(); err != nil {
		e.t.Fatalf("could not start htt: %v", err)
	}
	return p
}

// wait waits for p and fails the test if it didn't exit 0.
func (p *started) wait(t *testing.T) {
	t.Helper()
	if err := p.cmd.Wait(); err != nil {
		t.Errorf("htt %s: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(p.args, " "), err, p.stdout.String(), p.stderr.String())
	}
}

// concurrently is how many htt processes the tests run at once.
const concurrently = 20

// Twenty htt processes adding a task each to one context at once: all
// twenty tasks land. Before safe writes, each read the file, added its
// task and wrote the file back, so a process that read before another
// wrote discarded that one's task (and a reader could catch the file
// renamed away or half-written, and write that back).
func TestSafeWrites_ConcurrentAddsAllLand(t *testing.T) {
	e := safeEnv(t)
	e.mustRun("todo", "add", "already there")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var procs []*started
	for i := range concurrently {
		procs = append(procs, e.start(ctx, "todo", "add", fmt.Sprintf("added by process %02d", i)))
	}
	for _, p := range procs {
		p.wait(t)
	}

	lines := strings.Split(strings.TrimSuffix(e.readData("todo.txt"), "\n"), "\n")
	var lost []string
	for i := range concurrently {
		want := fmt.Sprintf("added by process %02d", i)
		if n := countSuffix(lines, " "+want); n != 1 {
			lost = append(lost, fmt.Sprintf("%q %d times", want, n))
		}
	}
	if countSuffix(lines, " already there") != 1 {
		lost = append(lost, `"already there" gone`)
	}
	if len(lost) > 0 {
		t.Errorf("%d of %d tasks lost or doubled: %s\ntodo.txt:\n%s", len(lost), concurrently+1, strings.Join(lost, "; "), strings.Join(lines, "\n"))
	}
	if len(lines) != concurrently+1 {
		t.Errorf("todo.txt has %d lines, want %d", len(lines), concurrently+1)
	}
}

// Processes completing, adding and raising priorities in one context at
// once: every task lands where its command put it. A completion moves a
// task to done.txt, so two files change in one go.
func TestSafeWrites_ConcurrentChangesAcrossFilesAllLand(t *testing.T) {
	e := safeEnv(t)
	var seed strings.Builder
	for i := range concurrently {
		fmt.Fprintf(&seed, "finish %02d\n", i)
	}
	e.writeData("todo.txt", seed.String())

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var procs []*started
	for i := range concurrently {
		// todo do takes a number, read when the command runs; the
		// first task is always one of the "finish" tasks still open.
		procs = append(procs,
			e.start(ctx, "todo", "do", "0"),
			e.start(ctx, "todo", "add", fmt.Sprintf("new %02d", i)))
	}
	for _, p := range procs {
		p.wait(t)
	}

	todo := strings.Split(strings.TrimSuffix(e.readData("todo.txt"), "\n"), "\n")
	done := strings.Split(strings.TrimSuffix(e.readData("done.txt"), "\n"), "\n")
	for i := range concurrently {
		if n := countSuffix(todo, fmt.Sprintf(" new %02d", i)); n != 1 {
			t.Errorf("new %02d is in todo.txt %d times, want once", i, n)
		}
		finish := fmt.Sprintf("finish %02d", i)
		inTodo := slices.Contains(todo, finish)
		inDone := slices.ContainsFunc(done, func(l string) bool { return strings.Contains(l, " "+finish+" ") })
		if inTodo || !inDone {
			t.Errorf("%q: in todo.txt %v, in done.txt %v; want it done, once", finish, inTodo, inDone)
		}
	}
	if len(todo) != concurrently || len(done) != concurrently {
		t.Errorf("todo.txt has %d lines and done.txt %d, want %d each\ntodo.txt:\n%s\ndone.txt:\n%s",
			len(todo), len(done), concurrently, strings.Join(todo, "\n"), strings.Join(done, "\n"))
	}
}

// Twenty htt processes logging an entry each at once: all twenty are in
// the day's log.
func TestSafeWrites_ConcurrentLogEntriesAllLand(t *testing.T) {
	e := safeEnv(t)
	e.mustRun("log", "add", "already logged")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var procs []*started
	for i := range concurrently {
		procs = append(procs, e.start(ctx, "log", "add", fmt.Sprintf("logged by process %02d", i)))
	}
	for _, p := range procs {
		p.wait(t)
	}

	// The day is the one the entries were logged on: around midnight,
	// some could fall on the next.
	var lines []string
	for _, day := range []time.Time{time.Now().AddDate(0, 0, -1), time.Now()} {
		if path := filepath.Join("timelogs", day.Format("2006-01-02")+".log"); e.dataExists(path) {
			lines = append(lines, strings.Split(strings.TrimSuffix(e.readData(path), "\n"), "\n")...)
		}
	}
	for i := range concurrently {
		entry := fmt.Sprintf("logged by process %02d ts:", i)
		if n := countContaining(lines, entry); n != 1 {
			t.Errorf("%q is in the log %d times, want once", entry, n)
		}
	}
	if n := countContaining(lines, "already logged ts:"); n != 1 {
		t.Errorf("the entry logged first is there %d times, want once", n)
	}
}

// Switching contexts again and again, htt never leaves the
// current-context pointer empty or half-written for a reader: it names
// one of the contexts each time.
func TestSafeWrites_ThePointerIsAlwaysWhole(t *testing.T) {
	e := safeEnv(t)
	e.mustRun("todo", "context", "first_context")
	path := filepath.Join(e.dataDir, "current-context")
	names := []string{"first_context", "second_context"}

	const switches = 30
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for i := range switches {
			if r := e.run("todo", "context", names[i%2]); r.exitCode != 0 {
				t.Errorf("switch %d: exit %d: %s", i, r.exitCode, r.stderr)
				return
			}
		}
	}()
	var reads, bad int
	var example string
	deadline := time.After(2 * time.Minute)
	for done := false; !done; reads++ {
		select {
		case <-finished:
			done = true
		case <-deadline:
			t.Fatal("the switches took over two minutes")
		default:
		}
		b, err := readAsAReader(path)
		if err != nil || !slices.Contains(names, string(b)) {
			bad++
			if example == "" {
				example = fmt.Sprintf("%q, %v", b, err)
			}
		}
	}
	if bad > 0 {
		t.Errorf("of %d reads, %d found the pointer missing or partial, as %s", reads, bad, example)
	}
	if e.dataExists("current-context.bak") {
		t.Errorf("the pointer got a .bak")
	}
}

func countContaining(lines []string, s string) int {
	n := 0
	for _, l := range lines {
		if strings.Contains(l, s) {
			n++
		}
	}
	return n
}

func countSuffix(lines []string, suffix string) int {
	n := 0
	for _, l := range lines {
		if strings.HasSuffix(l, suffix) {
			n++
		}
	}
	return n
}

// While htt rewrites a file again and again, a reader of it sees each
// time a whole version, the one before or the one after: never no file,
// never part of one. Before safe writes, htt renamed the file to .bak
// and wrote it again, a line at a time.
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
		b, err := readAsAReader(path)
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
