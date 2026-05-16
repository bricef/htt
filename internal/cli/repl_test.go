package cli

import (
	"slices"
	"testing"
)

func TestClassifyReplInput(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantKind replInputKind
		wantArgs []string
	}{
		{"empty", "", replInputNone, nil},
		{"whitespace only", "   \t  ", replInputNone, nil},
		{"slash with nothing after", "/", replInputNone, nil},
		{"slash + spaces", "/   ", replInputNone, nil},
		{"quit", "quit", replInputExit, nil},
		{"exit", "exit", replInputExit, nil},
		{"q", "q", replInputExit, nil},
		{"quit with surrounding ws", "  quit  ", replInputExit, nil},
		// Words that contain the exit prefix should NOT exit. We
		// match the exact full string.
		{"quitter is a task", "quitter", replInputDispatch, []string{"todo", "quitter"}},
		{"queue is a task", "queue stuff", replInputDispatch, []string{"todo", "queue", "stuff"}},

		// Todo-mode default
		{"add task", "add buy milk", replInputDispatch, []string{"todo", "add", "buy", "milk"}},
		{"delete index", "delete 0", replInputDispatch, []string{"todo", "delete", "0"}},
		{"priority change", "+ 1", replInputDispatch, []string{"todo", "+", "1"}},
		{"surrounding whitespace", "   add buy milk   ", replInputDispatch, []string{"todo", "add", "buy", "milk"}},

		// Slash escape
		{"slash log start", "/log start review PRs", replInputDispatch, []string{"log", "start", "review", "PRs"}},
		{"slash report", "/report --since 7d", replInputDispatch, []string{"report", "--since", "7d"}},
		{"slash trims rest", "/   log start", replInputDispatch, []string{"log", "start"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			kind, args := classifyReplInput(c.in)
			if kind != c.wantKind {
				t.Errorf("kind = %v, want %v", kind, c.wantKind)
			}
			if !slices.Equal(args, c.wantArgs) {
				t.Errorf("args = %v, want %v", args, c.wantArgs)
			}
		})
	}
}

func TestIsDisabledInRepl(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"empty args", nil, false},
		{"interactive blocked", []string{"interactive"}, true},
		{"repl blocked", []string{"repl"}, true},
		{"todo allowed", []string{"todo", "add", "foo"}, false},
		{"log allowed", []string{"log", "start"}, false},
		{"report allowed", []string{"report"}, false},
		{"sync allowed", []string{"sync"}, false},
		// Subcommand of an allowed command stays allowed.
		{"log status allowed", []string{"log", "status"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isDisabledInRepl(c.args); got != c.want {
				t.Errorf("isDisabledInRepl(%v) = %v, want %v", c.args, got, c.want)
			}
		})
	}
}

// TestRepl_DispatchExecutesCommand wires classifyReplInput to a
// real RootCmd.Execute pass and verifies the effect lands in the
// memory repo. This pins the end-to-end glue without needing a
// TTY for readline.
func TestRepl_DispatchExecutesCommand(t *testing.T) {
	repo := withMemoryRepo(t)

	kind, args := classifyReplInput("add buy milk")
	if kind != replInputDispatch {
		t.Fatalf("kind = %v, want dispatch", kind)
	}
	if isDisabledInRepl(args) {
		t.Fatalf("dispatch unexpectedly disabled for %v", args)
	}
	resetReplFlags()
	RootCmd.SetArgs(args)
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	ctx, _ := repo.Context("todo")
	if len(ctx.Tasks) != 1 || ctx.Tasks[0].Entry() != "buy milk" {
		t.Errorf("repo state = %v, want one 'buy milk'", ctx.Tasks)
	}
}

func TestResetReplFlags_ClearsBleedingState(t *testing.T) {
	// Set flag globals to non-default values, then verify
	// resetReplFlags restores them. This is the cycle-to-cycle
	// hygiene that prevents prior --due / --since values from
	// poisoning the next dispatch.
	addDue = "Friday"
	addToDue = "tomorrow"
	reportSince = "yesterday"

	resetReplFlags()

	if addDue != "" {
		t.Errorf("addDue = %q, want empty", addDue)
	}
	if addToDue != "" {
		t.Errorf("addToDue = %q, want empty", addToDue)
	}
	if reportSince != "7d" {
		t.Errorf("reportSince = %q, want 7d", reportSince)
	}
}
