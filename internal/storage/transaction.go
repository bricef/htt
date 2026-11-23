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
//  4. Check that no file it read has changed on disk since (another
//     program wrote it). If one has, go back to 2; after a few runs,
//     give up, writing nothing.
//  5. Write each file the change wrote, in the order it wrote them,
//     atomically: a temporary file beside it, synced, renamed over it.
//     The previous version is kept as .bak.
//  6. Unlock.
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
	"runtime"
	"slices"
	"strconv"
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
	// underneath runs after each run of the change and before the
	// check, with the run's number from 1: a test changes a file there
	// as another program would.
	underneath func(run int)
	// replacing runs before each file a transaction writes is
	// replaced; an error from it fails the write there, as a full disk
	// would, leaving that file and those after it as they were.
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
// in the order.
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
// reads until nothing it read has changed underneath, a few runs at
// most, then write and unlock. An error from change writes nothing and
// is returned as it is. Another htt holding the lock too long is
// domain.ErrBusy; a file changing under every run is
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
		if h.underneath != nil {
			h.underneath(run)
		}
		changed, err := t.changed()
		if err != nil {
			return err
		}
		if changed == "" {
			return t.commit(dirs, h)
		}
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

// commit writes the transaction's files in the order the change wrote
// them, keeping each one's previous version as .bak where asked. The
// change wrote the file gaining tasks before the one losing them, so a
// failure part-way leaves a task in both, never in neither.
func (t *transaction) commit(dirs []string, h hooks) error {
	for _, w := range t.writes {
		if !slices.ContainsFunc(dirs, func(d string) bool { return filepath.Clean(d) == filepath.Dir(w.path) }) {
			return fmt.Errorf("write %s: outside the directories locked (%v)", w.path, dirs)
		}
	}
	for _, w := range t.writes {
		if h.replacing != nil {
			if err := h.replacing(w.path); err != nil {
				return err
			}
		}
		if w.bak {
			was, ok := t.read[w.path] // checked: still the file's
			if !ok {                  // written without being read
				var err error
				if was, err = readFile(w.path); err != nil {
					return err
				}
			}
			if was.exists {
				if err := replace(w.path+".bak", was.data); err != nil {
					return fmt.Errorf("back up %s: %w", w.path, err)
				}
			}
		}
		if err := replace(w.path, w.data); err != nil {
			return err
		}
	}
	return nil
}

// readFile reads path whole. A file that doesn't exist isn't an error:
// its snapshot has exists false.
func readFile(path string) (snapshot, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return snapshot{}, nil
	}
	if err != nil {
		return snapshot{}, fmt.Errorf("read %s: %w", path, err)
	}
	return snapshot{data: b, exists: true}, nil
}

// replace makes data path's content atomically: it goes to a temporary
// file beside path, which is synced and renamed over path, so a reader
// sees the old file or the new one and never anything in between. A
// failure leaves path as it was and no temporary file behind.
func replace(path string, data []byte) error {
	f, err := createTemp(path)
	if err != nil {
		return err
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
		if err := f.Close(); err != nil {
			return err
		}
		return rename(tmp, path)
	}()
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

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
