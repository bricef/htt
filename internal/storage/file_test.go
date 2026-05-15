package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bricef/htt/internal/domain"
)

func TestFileRepository_Contract(t *testing.T) {
	runRepositoryContract(t, func(t *testing.T) domain.Repository {
		return NewFileRepository(t.TempDir(), t.TempDir())
	})
}

func TestFileRepository_Context_DoesNotWriteOnRead(t *testing.T) {
	// Pin the read-no-longer-writes fix: opening a context must not
	// touch the file. This is the bug that lived in todo.Context.Read
	// (it called Add which called Sync) — now corrected.
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	if err := os.WriteFile(path, []byte("a\nb\n"), 0644); err != nil {
		t.Fatal(err)
	}
	info1, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	r := NewFileRepository(dir, dir)
	if _, err := r.Context("todo"); err != nil {
		t.Fatalf("Context: %v", err)
	}

	info2, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info2.ModTime().Equal(info1.ModTime()) {
		t.Errorf("Context modified file mtime: before=%v after=%v", info1.ModTime(), info2.ModTime())
	}
	if info2.Size() != info1.Size() {
		t.Errorf("Context changed file size: before=%d after=%d", info1.Size(), info2.Size())
	}

	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Errorf("Context should not produce a .bak; got err=%v", err)
	}
}

func TestFileRepository_Save_CreatesBakBackup(t *testing.T) {
	// Behavior preservation: the legacy Sync() renamed the existing file
	// to .bak before writing. Preserve that side effect for users who may
	// rely on it. This is documented in the plan as "the well-known .bak
	// side effect" and is the only artifact of the legacy code path.
	dir := t.TempDir()
	r := NewFileRepository(dir, dir)

	first := &domain.Context{
		Name:  "todo",
		Tasks: []*domain.Task{mustTask(t, "v1")},
	}
	if err := r.Save(first); err != nil {
		t.Fatalf("first save: %v", err)
	}

	second := &domain.Context{
		Name:  "todo",
		Tasks: []*domain.Task{mustTask(t, "v2")},
	}
	if err := r.Save(second); err != nil {
		t.Fatalf("second save: %v", err)
	}

	bak, err := os.ReadFile(filepath.Join(dir, "todo.txt.bak"))
	if err != nil {
		t.Fatalf(".bak should exist after second save: %v", err)
	}
	if got := strings.TrimSpace(string(bak)); got != "v1" {
		t.Errorf(".bak content = %q, want %q", got, "v1")
	}

	current, err := os.ReadFile(filepath.Join(dir, "todo.txt"))
	if err != nil {
		t.Fatalf("todo.txt should exist: %v", err)
	}
	if got := strings.TrimSpace(string(current)); got != "v2" {
		t.Errorf("todo.txt content = %q, want %q", got, "v2")
	}
}

func TestFileRepository_Save_ByteExactOutput(t *testing.T) {
	// Pin the on-disk format. The legacy code writes one task per line,
	// each followed by '\n', with no header, footer, or trailing blank
	// lines. FileRepository must match byte-for-byte so existing on-disk
	// data is fully compatible.
	dir := t.TempDir()
	r := NewFileRepository(dir, dir)

	ctx := &domain.Context{
		Name: "todo",
		Tasks: []*domain.Task{
			mustTask(t, "(A) urgent"),
			mustTask(t, "buy milk"),
			mustTask(t, "x 2024-01-15 finished thing"),
		},
	}
	if err := r.Save(ctx); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "todo.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := "(A) urgent\nbuy milk\nx 2024-01-15 finished thing\n"
	if string(got) != want {
		t.Errorf("file content mismatch\n got: %q\nwant: %q", string(got), want)
	}
}

func TestFileRepository_Context_SkipsBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	body := "first\n\n  \nsecond\n"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewFileRepository(dir, dir)
	ctx, err := r.Context("todo")
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Tasks) != 2 {
		t.Fatalf("len(Tasks) = %d, want 2", len(ctx.Tasks))
	}
	if ctx.Tasks[0].Raw != "first" || ctx.Tasks[1].Raw != "second" {
		t.Errorf("unexpected tasks: %q, %q", ctx.Tasks[0].Raw, ctx.Tasks[1].Raw)
	}
}

func TestFileRepository_ContextNames_IgnoresNonTxtAndDirs(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"todo.txt", "work.txt", "done.txt", "todo.txt.bak", "current-context", "README.md"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	r := NewFileRepository(dir, dir)
	names, err := r.ContextNames()
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]bool{}
	for _, n := range names {
		got[n] = true
	}
	for _, want := range []string{"todo", "work", "done"} {
		if !got[want] {
			t.Errorf("missing context %q in %v", want, names)
		}
	}
	for _, unwanted := range []string{"current-context", "README", "subdir", "todo.txt"} {
		if got[unwanted] {
			t.Errorf("unexpected entry %q in %v", unwanted, names)
		}
	}
}

func TestFileRepository_ContextNames_OnMissingDir(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	r := NewFileRepository(missing, missing)
	names, err := r.ContextNames()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected empty list, got %v", names)
	}
}

func TestFileRepository_CurrentContextName_FromExistingPointer(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "current-context"), []byte("work\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r := NewFileRepository(dir, dir)
	name, err := r.CurrentContextName()
	if err != nil {
		t.Fatal(err)
	}
	if name != "work" {
		t.Errorf("got %q, want work", name)
	}
}

func TestFileRepository_CurrentPointer_LivesInPointerDir(t *testing.T) {
	// bug_005: SetCurrent / CurrentContextName must read and write the
	// pointer file under pointerDir, not dataDir. Legacy installs that
	// split tracker_path from data_path relied on this.
	dataDir := t.TempDir()
	pointerDir := t.TempDir()
	r := NewFileRepository(dataDir, pointerDir)

	if err := r.SetCurrent("work"); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}

	if _, err := os.Stat(filepath.Join(pointerDir, currentContextFile)); err != nil {
		t.Errorf("pointer file should exist in pointerDir, stat err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, currentContextFile)); !os.IsNotExist(err) {
		t.Errorf("pointer file should NOT be in dataDir, stat err: %v", err)
	}

	// Reading back returns what was persisted.
	name, err := r.CurrentContextName()
	if err != nil {
		t.Fatalf("CurrentContextName: %v", err)
	}
	if name != "work" {
		t.Errorf("got %q, want work", name)
	}
}

func TestFileRepository_Context_SanitizesNameAndStaysInDataDir(t *testing.T) {
	// bug_011: A name containing path separators or .. segments used to
	// flow straight into filepath.Join, letting Save("../escape") write
	// outside dataDir. Sanitization via utils.StringToFilename
	// (non-word → underscore) keeps every file inside dataDir.
	dataDir := t.TempDir()
	pointerDir := t.TempDir()
	r := NewFileRepository(dataDir, pointerDir)

	// Save through a name that would escape if unsanitized.
	if err := r.Save(&domain.Context{
		Name:  "../escapee",
		Tasks: []*domain.Task{mustTask(t, "task")},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// No file appeared one level up.
	parent := filepath.Dir(dataDir)
	if _, err := os.Stat(filepath.Join(parent, "escapee.txt")); err == nil {
		t.Errorf("file escaped dataDir at %s", filepath.Join(parent, "escapee.txt"))
	}

	// A sanitized version landed inside dataDir.
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if strings.Contains(e.Name(), "escapee") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a sanitized escapee*.txt file in %s, dir contents = %v", dataDir, entries)
	}
}

func TestFileRepository_ContextPath_BuildsExpectedLayout(t *testing.T) {
	// Plain name → <dataDir>/<name>.txt.
	dir := t.TempDir()
	r := NewFileRepository(dir, dir)

	got := r.ContextPath("work")
	want := filepath.Join(dir, "work.txt")
	if got != want {
		t.Errorf("ContextPath(\"work\") = %q, want %q", got, want)
	}
}

func TestFileRepository_ContextPath_SanitizesAndStaysInDataDir(t *testing.T) {
	// Path returned for a hostile name must stay inside dataDir, same
	// guarantee Save / Context already give. Mirrors the bug_011 test.
	dir := t.TempDir()
	r := NewFileRepository(dir, dir)

	got := r.ContextPath("../escape")
	if !strings.HasPrefix(got, dir+string(filepath.Separator)) {
		t.Errorf("ContextPath(\"../escape\") = %q, expected to start with %q/", got, dir)
	}
}

func TestMemoryRepository_ContextPath_ReturnsEmpty(t *testing.T) {
	// Memory repo has no filesystem path; callers that need a usable
	// path (htt todo edit-done, TUI EditFile) treat empty as
	// "unsupported by this repo".
	r := NewMemoryRepository()
	if got := r.ContextPath("anything"); got != "" {
		t.Errorf("memory ContextPath should be empty, got %q", got)
	}
}
