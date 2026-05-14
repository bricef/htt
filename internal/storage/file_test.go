package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bricef/htt/internal/domain"
)

func TestFileRepository_Contract(t *testing.T) {
	runRepositoryContract(t, func(t *testing.T) Repository {
		return NewFileRepository(t.TempDir())
	})
}

func TestFileRepository_LoadContext_DoesNotWriteOnRead(t *testing.T) {
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

	r := NewFileRepository(dir)
	if _, err := r.LoadContext("todo"); err != nil {
		t.Fatalf("LoadContext: %v", err)
	}

	info2, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info2.ModTime().Equal(info1.ModTime()) {
		t.Errorf("LoadContext modified file mtime: before=%v after=%v", info1.ModTime(), info2.ModTime())
	}
	if info2.Size() != info1.Size() {
		t.Errorf("LoadContext changed file size: before=%d after=%d", info1.Size(), info2.Size())
	}

	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Errorf("LoadContext should not produce a .bak; got err=%v", err)
	}
}

func TestFileRepository_SaveContext_CreatesBakBackup(t *testing.T) {
	// Behavior preservation: the legacy Sync() renamed the existing file
	// to .bak before writing. Preserve that side effect for users who may
	// rely on it. This is documented in the plan as "the well-known .bak
	// side effect" and is the only artifact of the legacy code path.
	dir := t.TempDir()
	r := NewFileRepository(dir)

	first := &domain.Context{
		Name:  "todo",
		Tasks: []*domain.Task{mustTask(t,"v1")},
	}
	if err := r.SaveContext(first); err != nil {
		t.Fatalf("first save: %v", err)
	}

	second := &domain.Context{
		Name:  "todo",
		Tasks: []*domain.Task{mustTask(t,"v2")},
	}
	if err := r.SaveContext(second); err != nil {
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

func TestFileRepository_SaveContext_ByteExactOutput(t *testing.T) {
	// Pin the on-disk format. The legacy code writes one task per line,
	// each followed by '\n', with no header, footer, or trailing blank
	// lines. FileRepository must match byte-for-byte so existing on-disk
	// data is fully compatible.
	dir := t.TempDir()
	r := NewFileRepository(dir)

	ctx := &domain.Context{
		Name: "todo",
		Tasks: []*domain.Task{
			mustTask(t,"(A) urgent"),
			mustTask(t,"buy milk"),
			mustTask(t,"x 2024-01-15 finished thing"),
		},
	}
	if err := r.SaveContext(ctx); err != nil {
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

func TestFileRepository_LoadContext_SkipsBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	body := "first\n\n  \nsecond\n"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewFileRepository(dir)
	ctx, err := r.LoadContext("todo")
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

func TestFileRepository_ListContexts_IgnoresNonTxtAndDirs(t *testing.T) {
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

	r := NewFileRepository(dir)
	names, err := r.ListContexts()
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

func TestFileRepository_ListContexts_OnMissingDir(t *testing.T) {
	r := NewFileRepository(filepath.Join(t.TempDir(), "does-not-exist"))
	names, err := r.ListContexts()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected empty list, got %v", names)
	}
}

func TestFileRepository_GetCurrentContextName_FromExistingPointer(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "current-context"), []byte("work\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r := NewFileRepository(dir)
	name, err := r.GetCurrentContextName()
	if err != nil {
		t.Fatal(err)
	}
	if name != "work" {
		t.Errorf("got %q, want work", name)
	}
}
