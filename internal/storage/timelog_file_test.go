package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/vars"
)

func TestFileTimelogRepository_Contract(t *testing.T) {
	runTimelogRepositoryContract(t, func(t *testing.T) domain.TimelogRepository {
		return NewFileTimelogRepository(t.TempDir())
	})
}

func TestFileTimelogRepository_CurrentLogPath_UsesViperLayout(t *testing.T) {
	// Pins the on-disk layout: <dataDir>/<DefaultTimelogDirName>/<YYYY-MM-DD>.log
	dir := t.TempDir()
	r := NewFileTimelogRepository(dir)

	got := r.CurrentLogPath()
	today := time.Now().Format("2006-01-02")
	want := filepath.Join(dir, vars.DefaultTimelogDirName, today+".log")
	if got != want {
		t.Errorf("CurrentLogPath = %q, want %q", got, want)
	}
}

func TestFileTimelogRepository_Today_OnMissingFile(t *testing.T) {
	// A fresh install (no timelog files yet) reads as an empty Timelog,
	// not an error. Mirrors the legacy utils.ReadLines short-circuit
	// behaviour and the bug_004 nil-safe path.
	r := NewFileTimelogRepository(t.TempDir())

	l, err := r.Today()
	if err != nil {
		t.Fatalf("Today on missing file: %v", err)
	}
	if !l.IsEmpty() {
		t.Errorf("missing-file Timelog should be empty, got %v", l.Entries)
	}
}

func TestFileTimelogRepository_Save_ByteExactOutput(t *testing.T) {
	// One entry per line, '\n' terminated, no header, no footer.
	// Matches what the legacy timelogs.AddEntry produced.
	dir := t.TempDir()
	r := NewFileTimelogRepository(dir)

	date := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	l := &domain.Timelog{
		Date: date,
		Entries: []*domain.Task{
			mustTask(t, "wrote code ts:2026-05-15T09:00:00Z"),
			mustTask(t, "took a break ts:2026-05-15T10:30:00Z"),
			mustTask(t, "@end ts:2026-05-15T17:00:00Z"),
		},
	}
	if err := r.Save(l); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path := filepath.Join(dir, vars.DefaultTimelogDirName, "2026-05-15.log")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "wrote code ts:2026-05-15T09:00:00Z\n" +
		"took a break ts:2026-05-15T10:30:00Z\n" +
		"@end ts:2026-05-15T17:00:00Z\n"
	if string(got) != want {
		t.Errorf("file content mismatch\n got: %q\nwant: %q", string(got), want)
	}
}

func TestFileTimelogRepository_Save_CreatesBakBackup(t *testing.T) {
	// Parallels TestFileRepository_Save_CreatesBakBackup. Save renames
	// the existing file to .bak before writing, giving one slot of
	// crash-recovery history.
	dir := t.TempDir()
	r := NewFileTimelogRepository(dir)
	date := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)

	first := &domain.Timelog{Date: date, Entries: []*domain.Task{mustTask(t, "v1 ts:2026-05-15T09:00:00Z")}}
	if err := r.Save(first); err != nil {
		t.Fatalf("first save: %v", err)
	}

	second := &domain.Timelog{Date: date, Entries: []*domain.Task{mustTask(t, "v2 ts:2026-05-15T10:00:00Z")}}
	if err := r.Save(second); err != nil {
		t.Fatalf("second save: %v", err)
	}

	bakPath := filepath.Join(dir, vars.DefaultTimelogDirName, "2026-05-15.log.bak")
	bak, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf(".bak should exist after second save: %v", err)
	}
	if got := strings.TrimSpace(string(bak)); got != "v1 ts:2026-05-15T09:00:00Z" {
		t.Errorf(".bak content = %q", got)
	}

	currentPath := filepath.Join(dir, vars.DefaultTimelogDirName, "2026-05-15.log")
	current, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("current file: %v", err)
	}
	if got := strings.TrimSpace(string(current)); got != "v2 ts:2026-05-15T10:00:00Z" {
		t.Errorf("current content = %q", got)
	}
}

func TestFileTimelogRepository_Day_SkipsBlankLines(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, vars.DefaultTimelogDirName), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, vars.DefaultTimelogDirName, "2026-05-15.log")
	body := "first ts:2026-05-15T09:00:00Z\n\n  \nsecond ts:2026-05-15T10:00:00Z\n"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewFileTimelogRepository(dir)
	l, err := r.Day(time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2 (blanks skipped)", len(l.Entries))
	}
}
