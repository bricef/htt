package storage

// Safe writes (docs/design/safe-writes.md). Every write htt makes to its
// files runs as a transaction, here, and nothing else in htt locks a
// file, writes a temporary one or renames one over another:
//
//  1. Lock: take htt's lock in each directory the transaction can write,
//     waiting a few seconds at most for another htt to let go of it.
//  2. Read the files the change asks for, fresh from disk, keeping what
//     was read.
//  3. Change: run the change on what was read; its writes are held.
//  4. Write: each file the change wrote, and the previous version of
//     each, its .bak, goes to a temporary file beside it, written and
//     synced.
//  5. Check that no file it read has changed on disk since (another
//     program wrote it). If one has, remove the temporary files and go
//     back to 2; after a few runs, give up, writing nothing.
//  6. Replace: rename each temporary file over its file, the files
//     gaining tasks before those losing them, so a reader sees the old
//     file or the new one, and a failure part-way leaves a task in
//     both, never in neither. Only renames follow the check, so the
//     moment in which another program's write can go unseen is as
//     short as it can be.
//  7. Unlock.
//
// The repositories (file.go, timelog_file.go) only parse what a
// transaction reads and serialise what they save.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bricef/htt/internal/domain"
	"github.com/gofrs/flock"
)

// lockName is the file in each directory htt writes that holds htt's
// lock. It ends in neither .txt nor .log, so it is never taken for a
// context or a day's log.
const lockName = ".htt.lock"

// runs is how many times a transaction runs its change before a file
// that keeps changing under it makes it give up.
const runs = 5

// lockWait is how long a transaction waits for another htt to let go
// of the lock before giving up with domain.ErrBusy. A variable so that
// storage's tests can shorten it.
var lockWait = 5 * time.Second

// lockPoll is how often a waiting transaction tries the lock again.
const lockPoll = 5 * time.Millisecond

// renameWait bounds the retries of a rename on Windows (see rename).
const renameWait = 2 * time.Second

// hooks are seams for storage's own tests. The zero value has none.
type hooks struct {
	// writing runs before each temporary file is written, with the path
	// it is for (a file the change wrote, or its .bak); an error from it
	// fails the write there, as a full disk would, and nothing is
	// replaced.
	writing func(path string) error
	// underneath runs after each run of the change, its files written,
	// and before the check, with the run's number from 1: a test changes
	// a file there as another program would.
	underneath func(run int)
	// replacing runs before each file the change wrote is replaced,
	// after the check (its .bak just before it); an error from it fails
	// the rename there, leaving that file and those after it as they
	// were.
	replacing func(path string) error
}

// transaction is one run of a change: the files it read, as it read
// them, and the writes it holds until the change returns.
type transaction struct {
	read   map[string]snapshot
	writes []write // in the order the change made them
}

// snapshot is a file as a transaction read it. A file that doesn't
// exist has exists false.
type snapshot struct {
	data   []byte
	exists bool
}

// write is a file's new content, held until the change returns. bak
// keeps the file's previous version as <path>.bak.
type write struct {
	path string
	data []byte
	bak  bool
}

// Read returns path's content as the transaction sees it: what the
// change wrote there, if it did; otherwise the file as the transaction
// first read it, reading it now the first time. A file that doesn't
// exist has exists false.
func (t *transaction) Read(path string) (data []byte, exists bool, err error) {
	if i := t.written(path); i >= 0 {
		return t.writes[i].data, true, nil
	}
	if s, ok := t.read[path]; ok {
		return s.data, s.exists, nil
	}
	s, err := readFile(path)
	if err != nil {
		return nil, false, err
	}
	t.read[path] = s
	return s.data, s.exists, nil
}

// Write holds data as path's new content, written when the change
// returns, the file's previous version kept as <path>.bak when bak is
// true (task files and logs; the current-context pointer has none).
// Writing a path again replaces the content and keeps the path's place
// in the order the change wrote them, which the replacing follows among
// files alike (gainingFirst).
func (t *transaction) Write(path string, data []byte, bak bool) {
	if i := t.written(path); i >= 0 {
		t.writes[i].data = data
		return
	}
	t.writes = append(t.writes, write{path: path, data: data, bak: bak})
}

// Paths returns the paths the change has written so far, in order.
func (t *transaction) Paths() []string {
	out := make([]string, len(t.writes))
	for i, w := range t.writes {
		out[i] = w.path
	}
	return out
}

func (t *transaction) written(path string) int {
	return slices.IndexFunc(t.writes, func(w write) bool { return w.path == path })
}

// transact runs change as one transaction over the files in dirs, which
// are the directories it may write: lock, then run change on fresh
// reads and write what it wrote to temporary files, until nothing it
// read has changed underneath, a few runs at most; then rename the
// temporary files over their files, and unlock. An error from change
// writes nothing and is returned as it is. Another htt holding the lock
// too long is domain.ErrBusy; a file changing under every run is
// domain.ErrKeepsChanging, naming the file.
func transact(dirs []string, h hooks, change func(t *transaction) error) error {
	unlock, err := lock(dirs)
	if err != nil {
		return err
	}
	defer unlock()
	for run := 1; ; run++ {
		t := &transaction{read: map[string]snapshot{}}
		if err := change(t); err != nil {
			return err
		}
		if len(t.writes) == 0 {
			return nil // nothing to lose
		}
		files, err := t.stage(dirs, h)
		if err != nil {
			return err
		}
		if h.underneath != nil {
			h.underneath(run)
		}
		changed, err := t.changed()
		if err != nil {
			files.discard()
			return err
		}
		if changed == "" {
			return files.replace(h)
		}
		files.discard()
		if run == runs {
			return fmt.Errorf("%s %w", changed, domain.ErrKeepsChanging)
		}
	}
}

// changed returns the first file the transaction read whose content on
// disk isn't what it read any more, or "".
func (t *transaction) changed() (string, error) {
	paths := make([]string, 0, len(t.read))
	for p := range t.read {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range paths {
		now, err := readFile(p)
		if err != nil {
			return "", err
		}
		was := t.read[p]
		if now.exists != was.exists || !bytes.Equal(now.data, was.data) {
			return p, nil
		}
	}
	return "", nil
}

// stage writes each file the change wrote, the file gaining tasks first
// (gainingFirst), to a temporary file beside it, and the file's
// previous version to one beside its .bak where asked, each synced:
// everything but the renames, so before the check. The .bak is the file
// as the transaction read it, which the check makes sure is still the
// file's; a file written without being read is read now, and checked
// too. A failure removes what was written and returns the error. On the
// way, what a crashed htt left in the directories written is swept
// (sweep).
func (t *transaction) stage(dirs []string, h hooks) (staged, error) {
	for _, w := range t.writes {
		if !slices.ContainsFunc(dirs, func(d string) bool { return filepath.Clean(d) == filepath.Dir(w.path) }) {
			return nil, fmt.Errorf("write %s: outside the directories locked (%v)", w.path, dirs)
		}
	}
	for _, w := range t.writes {
		if _, ok := t.read[w.path]; !ok && w.bak { // written without being read: checked from now on
			was, err := readFile(w.path)
			if err != nil {
				return nil, err
			}
			t.read[w.path] = was
		}
	}
	var files staged
	swept := map[string]bool{}
	add := func(w write, bak bool, data []byte) error {
		path := w.path
		if bak {
			path += ".bak"
		}
		if h.writing != nil {
			if err := h.writing(path); err != nil {
				return err
			}
		}
		real, err := realPath(path)
		if err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		if dir := filepath.Dir(real); !swept[dir] {
			swept[dir] = true
			sweep(dir)
		}
		f, err := stage(real, data)
		if err != nil {
			return err
		}
		f.of, f.bak = w.path, bak
		files = append(files, f)
		return nil
	}
	for _, w := range t.gainingFirst() {
		if was := t.read[w.path]; w.bak && was.exists {
			if err := add(w, true, was.data); err != nil {
				files.discard()
				return nil, fmt.Errorf("back up %s: %w", w.path, err)
			}
		}
		if err := add(w, false, w.data); err != nil {
			files.discard()
			return nil, err
		}
	}
	return files, nil
}

// gainingFirst returns the transaction's writes with the files that
// keep every line they had (they gain tasks, or are new) before those
// that lose a line (they lose a task, or a task changed), each group in
// the order the change wrote them. The domain saves the file gaining a
// task before the one losing it, but a body composing operations
// (Context.Transact) can save them the other way round, adding a task
// to a file before completing one from it; whatever the order of the
// saves, a failure part-way then leaves a task in both files, never in
// neither. Lines are compared as the repositories read them: trimmed,
// blank ones left out.
func (t *transaction) gainingFirst() []write {
	loses := func(w write) bool {
		was, ok := t.read[w.path]
		if !ok || !was.exists {
			return false
		}
		left := map[string]int{}
		for l := range strings.Lines(string(was.data)) {
			if l = strings.TrimSpace(l); l != "" {
				left[l]++
			}
		}
		for l := range strings.Lines(string(w.data)) {
			if l = strings.TrimSpace(l); left[l] > 0 {
				left[l]--
			}
		}
		for _, n := range left {
			if n > 0 {
				return true
			}
		}
		return false
	}
	out := make([]write, 0, len(t.writes))
	var losing []write
	for _, w := range t.writes {
		if loses(w) {
			losing = append(losing, w)
		} else {
			out = append(out, w)
		}
	}
	return append(out, losing...)
}

// realPath is the file a write to path replaces. A symlink (to a file a
// sync tool keeps elsewhere, say) is followed, so that the rename
// replaces the file it points to, beside which the temporary file is
// written, keeping the rename on that file's file system, and the link
// stays a link; renaming over the link itself would replace it with a
// regular file. A link to a file not there yet names where it will be.
func realPath(path string) (string, error) {
	for range 40 { // links followed at most, as on Linux
		fi, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			return path, nil
		}
		if err != nil {
			return "", err
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			return path, nil
		}
		if real, err := filepath.EvalSymlinks(path); err == nil {
			return real, nil
		}
		to, err := os.Readlink(path) // pointing to nothing yet
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(to) {
			to = filepath.Join(filepath.Dir(path), to)
		}
		path = to
	}
	return "", fmt.Errorf("%s: too many links", path)
}

// leftoverAge is how old a temporary file of htt's must be for a
// transaction to take it for one a crashed htt left behind: a
// transaction takes moments, and no htt holds the lock for an hour.
const leftoverAge = time.Hour

// leftover matches the temporary files createTemp names: a dot, the
// name of a file of htt's (a context or the done file, a day's log,
// either's .bak, the current-context pointer), a random number, .tmp.
var leftover = regexp.MustCompile(`^\.(.+\.(txt|log)(\.bak)?|` + currentContextFile + `)\.[0-9a-z]{1,13}\.tmp$`)

// sweep removes from dir the temporary files a crashed htt left behind
// (leftover), at least leftoverAge old. It runs with the lock held,
// before a transaction writes in dir; a file it can't remove waits for
// the next one.
func sweep(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !leftover.MatchString(e.Name()) {
			continue
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) >= leftoverAge {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// staged are the temporary files of a transaction's writes, in the
// order they're replaced, each file's .bak just before it.
type staged []stagedFile

// replace renames each temporary file over its file, in order: the file
// gaining tasks before the one losing them (gainingFirst), so a failure
// part-way leaves a task in both, never in neither. A failure removes
// the temporary files not yet renamed.
func (files staged) replace(h hooks) error {
	for i, f := range files {
		first := i == 0 || files[i-1].of != f.of // the write's .bak, or the file
		if h.replacing != nil && first {
			if err := h.replacing(f.of); err != nil {
				files[i:].discard()
				return err
			}
		}
		if err := f.replace(); err != nil {
			files[i+1:].discard()
			if f.bak {
				return fmt.Errorf("back up %s: %w", f.of, err)
			}
			return err
		}
	}
	return nil
}

// discard removes the temporary files, replacing nothing.
func (files staged) discard() {
	for _, f := range files {
		f.discard()
	}
}

// readFile reads path whole. A file that doesn't exist isn't an error:
// its snapshot has exists false. On Windows, opening a file at the
// moment another process renames a new one over it fails (a sharing
// violation), so there, as for rename, it's tried again for a couple of
// seconds.
func readFile(path string) (snapshot, error) {
	deadline := time.Now().Add(renameWait)
	b, err := os.ReadFile(path)
	for err != nil && runtime.GOOS == "windows" && heldOpen(err) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		b, err = os.ReadFile(path)
	}
	if errors.Is(err, os.ErrNotExist) {
		return snapshot{}, nil
	}
	if err != nil {
		return snapshot{}, fmt.Errorf("read %s: %w", path, err)
	}
	return snapshot{data: b, exists: true}, nil
}

// stagedFile is new content for path, in a temporary file beside it,
// written and synced, for replace to rename over path. of is the file
// the change wrote, path itself or, for its .bak (bak), the file it
// backs up.
type stagedFile struct {
	tmp, path, of string
	bak           bool
}

// stage writes data to a temporary file beside path and syncs it, so
// that renaming it over path (replace) makes data path's content
// atomically: a reader sees the old file or the new one and never
// anything in between. A failure leaves no temporary file behind.
func stage(path string, data []byte) (stagedFile, error) {
	f, err := createTemp(path)
	if err != nil {
		return stagedFile{}, err
	}
	tmp := f.Name()
	err = func() error {
		if _, err := f.Write(data); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Sync(); err != nil {
			_ = f.Close()
			return err
		}
		return f.Close()
	}()
	if err != nil {
		_ = os.Remove(tmp)
		return stagedFile{}, fmt.Errorf("write %s: %w", path, err)
	}
	return stagedFile{tmp: tmp, path: path, of: path}, nil
}

// replace renames the temporary file over its path. A failure leaves
// the path as it was and no temporary file behind.
func (f stagedFile) replace() error {
	if err := rename(f.tmp, f.path); err != nil {
		f.discard()
		return fmt.Errorf("write %s: %w", f.path, err)
	}
	return nil
}

// discard removes the temporary file.
func (f stagedFile) discard() { _ = os.Remove(f.tmp) }

// createTemp creates a new file beside path, named after it
// (.<name>.<random>.tmp, which no reader of htt's files takes for a
// context or a log), with the mode htt's files have always had: the
// umask applies, as it did when htt created the file itself.
func createTemp(path string) (*os.File, error) {
	dir, name := filepath.Split(path)
	for range 100 {
		tmp := filepath.Join(dir, "."+name+"."+strconv.FormatUint(rand.Uint64(), 36)+".tmp")
		f, err := os.OpenFile(tmp, os.O_RDWR|os.O_CREATE|os.O_EXCL, fileMode)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("create a temporary file for %s: %w", path, err)
		}
		return f, nil
	}
	return nil, fmt.Errorf("create a temporary file for %s: every name tried was taken", path)
}

// rename renames from over to. On Windows os.Rename is MoveFileEx with
// MOVEFILE_REPLACE_EXISTING, which replaces an existing file, but fails
// while another process has that file open (Go, htt included, opens
// files without FILE_SHARE_DELETE): access denied, or a sharing
// violation. Readers hold htt's files for moments, so there the rename
// is tried again for a couple of seconds.
func rename(from, to string) error {
	deadline := time.Now().Add(renameWait)
	for {
		err := os.Rename(from, to)
		if err == nil || runtime.GOOS != "windows" || !heldOpen(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// heldOpen reports whether err is Windows' ERROR_ACCESS_DENIED (5) or
// ERROR_SHARING_VIOLATION (32), what renaming over a file another
// process has open fails with there.
func heldOpen(err error) bool {
	var errno syscall.Errno
	return errors.As(err, &errno) && (errno == 5 || errno == 32)
}

// lock takes htt's lock in each of dirs, making them if need be, in a
// fixed order (by path) so two transactions can't wait on each other,
// within lockWait in all; unlock lets go of them. An operating-system
// lock goes with its process, so a crashed htt leaves none behind.
func lock(dirs []string) (unlock func(), err error) {
	var held []*flock.Flock
	unlock = func() {
		for i := len(held) - 1; i >= 0; i-- {
			_ = held[i].Unlock()
		}
	}
	ordered, err := lockOrder(dirs)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	for _, dir := range ordered {
		l := flock.New(filepath.Join(dir, lockName))
		ok, err := l.TryLockContext(ctx, lockPoll)
		if !ok {
			unlock()
			if err == nil || errors.Is(err, context.DeadlineExceeded) {
				return nil, domain.ErrBusy
			}
			return nil, fmt.Errorf("lock %s: %w", dir, err)
		}
		held = append(held, l)
	}
	return unlock, nil
}

// lockOrder returns dirs made, sorted, each directory once: two names
// for one directory (a symlink, a trailing separator) would otherwise
// have a transaction wait on its own lock.
func lockOrder(dirs []string) ([]string, error) {
	var out []string
	var seen []os.FileInfo
	for _, dir := range slices.Sorted(slices.Values(dirs)) {
		if err := os.MkdirAll(dir, dirMode); err != nil {
			return nil, fmt.Errorf("make %s: %w", dir, err)
		}
		fi, err := os.Stat(dir)
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(seen, func(s os.FileInfo) bool { return os.SameFile(s, fi) }) {
			continue
		}
		seen = append(seen, fi)
		out = append(out, dir)
	}
	return out, nil
}
