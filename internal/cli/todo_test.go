package cli

import (
	"strings"
	"testing"

	"github.com/bricef/htt/internal/storage"
	"github.com/bricef/htt/internal/vars"
	"github.com/spf13/viper"
)

// withMemoryRepo swaps the package-level Repository for a fresh in-memory
// instance and restores it on cleanup. Returns the underlying repo so the
// caller can inspect state.
//
// Tests using this helper must NOT run in parallel because RootCmd and
// the repo() injection point are package-level state.
func withMemoryRepo(t *testing.T) *storage.MemoryRepository {
	t.Helper()
	repo := storage.NewMemoryRepository()
	prev := defaultRepo
	SetRepository(repo)
	t.Cleanup(func() { SetRepository(prev) })
	return repo
}

// runCobra invokes RootCmd with the given args. Returns the error from
// Execute() — useful for verifying RunE error propagation. Stdout is not
// captured (commands use fmt.Printf directly); use the e2e harness for
// output assertions.
func runCobra(t *testing.T, args ...string) error {
	t.Helper()
	RootCmd.SetArgs(args)
	return RootCmd.Execute()
}

func TestCobra_AddCommand_PersistsToRepo(t *testing.T) {
	repo := withMemoryRepo(t)

	if err := runCobra(t, "todo", "add", "buy", "milk"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, err := repo.Context("todo")
	if err != nil {
		t.Fatalf("Context: %v", err)
	}
	if len(ctx.Tasks) != 1 || ctx.Tasks[0].Raw != "buy milk" {
		t.Errorf("repo state = %v, want one task 'buy milk'", ctx.Tasks)
	}
}

func TestCobra_DeleteOutOfRange_ReturnsError(t *testing.T) {
	withMemoryRepo(t)

	err := runCobra(t, "todo", "delete", "99")
	if err == nil {
		t.Fatalf("expected error from out-of-range delete, got nil")
	}
	if !strings.Contains(err.Error(), "delete task") {
		t.Errorf("error should mention delete task, got %q", err.Error())
	}
}

func TestCobra_InvalidPriority_ReturnsError(t *testing.T) {
	withMemoryRepo(t)

	if err := runCobra(t, "todo", "add", "task"); err != nil {
		t.Fatalf("add: %v", err)
	}

	err := runCobra(t, "todo", "priority", "0", "Z")
	if err == nil {
		t.Fatalf("expected error from invalid priority, got nil")
	}
	if !strings.Contains(err.Error(), "set priority") {
		t.Errorf("error should mention set priority, got %q", err.Error())
	}
}

func TestCobra_ContextSwitch_PersistsName(t *testing.T) {
	repo := withMemoryRepo(t)

	if err := runCobra(t, "todo", "context", "work"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	name, err := repo.CurrentContextName()
	if err != nil {
		t.Fatalf("CurrentContextName: %v", err)
	}
	if name != "work" {
		t.Errorf("current = %q, want work", name)
	}
}

func TestCobra_LogActive_HandlesEmptyLog(t *testing.T) {
	// bug_004: timelogs.CurrentActive returns nil for a missing/empty
	// log file. The old Active command dereferenced unconditionally and
	// panicked on a fresh install. Now it prints "No active task." and
	// returns cleanly.
	withMemoryRepo(t)
	dir := t.TempDir()
	prev := viper.Get(vars.ConfigKeyDataDir)
	viper.Set(vars.ConfigKeyDataDir, dir)
	t.Cleanup(func() { viper.Set(vars.ConfigKeyDataDir, prev) })

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("log active panicked on empty log: %v", r)
		}
	}()

	if err := runCobra(t, "log", "active"); err != nil {
		t.Errorf("log active should not error on empty log, got %v", err)
	}
}

func TestCobra_DoCommand_MovesToDone(t *testing.T) {
	repo := withMemoryRepo(t)
	if err := runCobra(t, "todo", "add", "make tea"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := runCobra(t, "todo", "do", "0"); err != nil {
		t.Fatalf("do: %v", err)
	}

	todoCtx, _ := repo.Context("todo")
	if len(todoCtx.Tasks) != 0 {
		t.Errorf("todo should be empty, got %v", todoCtx.Tasks)
	}
	doneCtx, _ := repo.Context("done")
	if len(doneCtx.Tasks) != 1 || !strings.Contains(doneCtx.Tasks[0].Raw, "make tea") {
		t.Errorf("done should contain completed task, got %v", doneCtx.Tasks)
	}
}
