package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/vars"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

		// Quotes group words, as in the shell; apostrophes stay.
		{"slash quoted --at", `/log end --at "yesterday 17:30"`, replInputDispatch, []string{"log", "end", "--at", "yesterday 17:30"}},
		{"slash --at= quoted", `/log add --at="2026-11-17 9:00" filing`, replInputDispatch, []string{"log", "add", "--at=2026-11-17 9:00", "filing"}},
		{"todo quoted word", `add "buy milk" today`, replInputDispatch, []string{"todo", "add", "buy milk", "today"}},
		{"todo apostrophe", "add call Sam's dentist", replInputDispatch, []string{"todo", "add", "call", "Sam's", "dentist"}},
		{"slash quoted help is help", `/help "x y"`, replInputHelp, nil},

		// /help builtin
		{"slash help", "/help", replInputHelp, nil},
		{"slash help with trailers", "/help extra args", replInputHelp, nil},
		{"slash help surrounded", "  /help  ", replInputHelp, nil},
		// Bare `help` (no slash) is just a todo dispatch — REPL help is /help only.
		{"bare help is a todo dispatch", "help", replInputDispatch, []string{"todo", "help"}},
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

func TestRefusedInRepl(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string // the command refused, "" when it runs
	}{
		{"empty args", nil, ""},
		{"repl refused", []string{"repl"}, "repl"},
		{"gui refused", []string{"gui"}, "gui"},
		{"gui with flags refused", []string{"gui", "--no-open"}, "gui"},
		// The TUI runs: the REPL hands it the terminal.
		{"interactive runs", []string{"interactive"}, ""},
		{"i runs", []string{"i"}, ""},
		{"interactive with flags runs", []string{"i", "--log-file", "tui.log"}, ""},
		{"todo runs", []string{"todo", "add", "foo"}, ""},
		{"log runs", []string{"log", "start"}, ""},
		{"report runs", []string{"report"}, ""},
		{"status runs", []string{"status"}, ""},
		// Subcommand of an allowed command stays allowed.
		{"log status runs", []string{"log", "status"}, ""},
		// Bare `i` in the REPL is `todo i`, which isn't the TUI.
		{"todo i runs", []string{"todo", "i"}, ""},
		{"unknown command runs, to fail", []string{"nonsense"}, ""},
	}
	// Each alias as its command: cobra finds them, so one added later
	// is covered too.
	for _, cmd := range []*cobra.Command{Interactive, Repl, GUI} {
		want := ""
		if cmd != Interactive {
			want = cmd.Name()
		}
		for _, alias := range cmd.Aliases {
			cases = append(cases, struct {
				name string
				args []string
				want string
			}{"alias " + alias + " of " + cmd.Name(), []string{alias}, want})
		}
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := refusedInRepl(c.args); got != c.want {
				t.Errorf("refusedInRepl(%q) = %q, want %q", c.args, got, c.want)
			}
		})
	}
}

// fakeLines is a line reader that records being closed in log.
type fakeLines struct {
	name string
	log  *[]string
}

func (f *fakeLines) Readline() (string, error) { return "", io.EOF }
func (f *fakeLines) SetPrompt(string)          {}
func (f *fakeLines) Close() error {
	*f.log = append(*f.log, "close "+f.name)
	return nil
}

// TestReplHandOver_ClosesRunsReopens checks the order the REPL hands
// its terminal to the TUI in: it closes its line reader, which would
// otherwise read the terminal too, runs the TUI, then opens a new
// reader, even when the TUI fails.
func TestReplHandOver_ClosesRunsReopens(t *testing.T) {
	for _, tuiErr := range []error{nil, errors.New("tui exited: boom")} {
		t.Run(fmt.Sprintf("tui returns %v", tuiErr), func(t *testing.T) {
			var log []string
			first := &fakeLines{name: "first", log: &log}
			second := &fakeLines{name: "second", log: &log}
			rt := &replTerminal{
				lines: first,
				open: func() (lineReader, error) {
					log = append(log, "open")
					return second, nil
				},
				isTerminal: func() bool { return true },
			}

			err := rt.handOver(func() error {
				log = append(log, "run the TUI")
				return tuiErr
			})

			if !errors.Is(err, tuiErr) {
				t.Errorf("handOver = %v, want the TUI's %v", err, tuiErr)
			}
			if want := []string{"close first", "run the TUI", "open"}; !slices.Equal(log, want) {
				t.Errorf("steps = %q, want %q", log, want)
			}
			if rt.lines != second {
				t.Errorf("the REPL reads with %v after the TUI, want the reader opened after it", rt.lines)
			}
		})
	}
}

// TestReplHandOver_NeedsATerminal: on a REPL reading a pipe, the TUI
// gets an error saying why it can't run, and the line reader is left
// as it is.
func TestReplHandOver_NeedsATerminal(t *testing.T) {
	var log []string
	first := &fakeLines{name: "first", log: &log}
	rt := &replTerminal{
		lines: first,
		open: func() (lineReader, error) {
			log = append(log, "open")
			return nil, errors.New("unexpected open")
		},
		isTerminal: func() bool { return false },
	}

	err := rt.handOver(func() error {
		log = append(log, "run the TUI")
		return nil
	})

	if err == nil || !strings.Contains(err.Error(), "the TUI needs a terminal") {
		t.Errorf("handOver = %v, want the TUI needs a terminal", err)
	}
	if len(log) != 0 || rt.lines != first {
		t.Errorf("steps = %q, reader %v; want none, and the first reader kept", log, rt.lines)
	}
}

// TestReplHandOver_ReopenFailureEndsTheREPL: when no new line reader
// opens after the TUI, the REPL's next read fails with the reason, which
// ends the REPL, rather than reading from the reader it closed.
func TestReplHandOver_ReopenFailureEndsTheREPL(t *testing.T) {
	var log []string
	rt := &replTerminal{
		lines: &fakeLines{name: "first", log: &log},
		open: func() (lineReader, error) {
			return nil, errors.New("no terminal left")
		},
		isTerminal: func() bool { return true },
	}

	if err := rt.handOver(func() error { return nil }); err != nil {
		t.Errorf("handOver = %v, want the TUI's nil", err)
	}
	if _, err := rt.lines.Readline(); err == nil || !strings.Contains(err.Error(), "no terminal left") {
		t.Errorf("Readline after a failed reopen = %v, want the open's error", err)
	}
}

// TestInteractive_RunsOnTheTerminalItIsGiven: htt interactive runs the
// TUI through runOnTerminal, which the REPL points at its handOver.
func TestInteractive_RunsOnTheTerminalItIsGiven(t *testing.T) {
	withMemoryRepo(t)
	refused := errors.New("no terminal for you")
	called := false
	prev := runOnTerminal
	runOnTerminal = func(run func() error) error {
		called = true
		return refused
	}
	t.Cleanup(func() { runOnTerminal = prev })

	resetReplFlags()
	RootCmd.SetArgs([]string{"i"})
	if err := RootCmd.Execute(); !errors.Is(err, refused) {
		t.Errorf("htt i = %v, want runOnTerminal's error", err)
	}
	if !called {
		t.Error("htt i didn't run the TUI through runOnTerminal")
	}
}

// TestRepl_DispatchExecutesCommand wires classifyReplInput to a
// real Execute pass, as the REPL runs a line, and verifies the
// effect lands in the memory repo. This pins the end-to-end glue
// without needing a TTY for readline.
func TestRepl_DispatchExecutesCommand(t *testing.T) {
	repo := withMemoryRepo(t)

	kind, args := classifyReplInput("add buy milk")
	if kind != replInputDispatch {
		t.Fatalf("kind = %v, want dispatch", kind)
	}
	if name := refusedInRepl(args); name != "" {
		t.Fatalf("dispatch unexpectedly refused %q for %v", name, args)
	}
	resetReplFlags()
	if err := Execute(args); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	ctx, _ := repo.Context("todo")
	if len(ctx.Tasks) != 1 || ctx.Tasks[0].Entry() != "buy milk" {
		t.Errorf("repo state = %v, want one 'buy milk'", ctx.Tasks)
	}
}

func TestResetReplFlags_ClearsReportPeriods(t *testing.T) {
	// --week, --month and whether --since was given: left set, the next
	// /report would be a week again, or refuse --since as one.
	reportWeek, reportMonth, reportSinceGiven = true, true, true
	resetReplFlags()
	if reportWeek || reportMonth || reportSinceGiven {
		t.Errorf("reportWeek %v, reportMonth %v, reportSinceGiven %v; want none set", reportWeek, reportMonth, reportSinceGiven)
	}
}

func TestResetReplFlags_ClearsBleedingState(t *testing.T) {
	// Set flag globals to non-default values, then verify
	// resetReplFlags restores them. This is the cycle-to-cycle
	// hygiene that prevents prior --due / --since values from
	// poisoning the next dispatch.
	addDue = "Friday"
	addToDue = "tomorrow"
	logAt = atFlag{value: "9:30", given: true}
	reportSince = "yesterday"
	showSortKey = "-due"
	showFilters = []string{"overdue"}
	tuiLogFile = "tui.log"
	tuiLogLevel = "error"
	guiPort = 8080
	guiNoOpen = true

	resetReplFlags()

	if addDue != "" {
		t.Errorf("addDue = %q, want empty", addDue)
	}
	if addToDue != "" {
		t.Errorf("addToDue = %q, want empty", addToDue)
	}
	if logAt != (atFlag{}) {
		t.Errorf("logAt = %+v, want it not given", logAt)
	}
	if reportSince != "7d" {
		t.Errorf("reportSince = %q, want 7d", reportSince)
	}
	if showSortKey != "" {
		t.Errorf("showSortKey = %q, want empty", showSortKey)
	}
	if showFilters != nil {
		t.Errorf("showFilters = %v, want nil", showFilters)
	}
	if tuiLogFile != "" {
		t.Errorf("tuiLogFile = %q, want empty", tuiLogFile)
	}
	if tuiLogLevel != "debug" {
		t.Errorf("tuiLogLevel = %q, want debug", tuiLogLevel)
	}
	if guiPort != 0 || guiNoOpen {
		t.Errorf("guiPort = %d, guiNoOpen = %v, want 0 and false", guiPort, guiNoOpen)
	}
}

// TestRepl_ShowFiltersDoNotBleedAcrossCycles runs two show commands
// the way the REPL loop does. pflag's StringSlice appends to the
// bound slice once the flag has been set in the process, so without
// the reset the second cycle would filter on both expressions.
func TestRepl_ShowFiltersDoNotBleedAcrossCycles(t *testing.T) {
	withMemoryRepo(t)

	for _, line := range []string{"show --filter overdue", "show --filter no-due"} {
		_, args := classifyReplInput(line)
		resetReplFlags()
		RootCmd.SetArgs(args)
		if err := RootCmd.Execute(); err != nil {
			t.Fatalf("%q: Execute: %v", line, err)
		}
	}

	if want := []string{"no-due"}; !slices.Equal(showFilters, want) {
		t.Errorf("showFilters after second cycle = %v, want %v", showFilters, want)
	}
}

// TestRepl_AtDoesNotBleedAcrossCycles runs two log adds the way the
// REPL loop does: the --at given to the first isn't the second's, which
// logs now, after it.
func TestRepl_AtDoesNotBleedAcrossCycles(t *testing.T) {
	withMemoryRepo(t)
	tlRepo := withMemoryTimelogRepo(t)
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	if now.Sub(midnight) < time.Minute || midnight.AddDate(0, 0, 1).Sub(now) < time.Minute {
		t.Skip("too close to midnight")
	}

	for _, line := range []string{"/log add --at 0:00 first", "/log add second"} {
		_, args := classifyReplInput(line)
		resetReplFlags()
		RootCmd.SetArgs(args)
		if err := RootCmd.Execute(); err != nil {
			t.Fatalf("%q: Execute: %v", line, err)
		}
	}

	tl, err := tlRepo.Today()
	if err != nil {
		t.Fatal(err)
	}
	stamp := func(at time.Time) string { return at.UTC().Format(time.RFC3339) }
	if len(tl.Entries) != 2 || tl.Entries[0].Raw != "first ts:"+stamp(midnight) ||
		!strings.HasPrefix(tl.Entries[1].Raw, "second ts:") || tl.Entries[1].Raw == "second ts:"+stamp(midnight) {
		t.Errorf("entries = %q, want first at midnight (%s), then second now", rawsOf(tl.Entries), stamp(midnight))
	}
}

func rawsOf(tasks []*domain.Task) []string {
	raws := make([]string, len(tasks))
	for i, task := range tasks {
		raws[i] = task.Raw
	}
	return raws
}

func TestReplHistoryPath_IsInTheConfigDirectory(t *testing.T) {
	// config_path names the config file. The history path used to be
	// built under it, as if it were a directory, which fails once the
	// file exists and switched history off.
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	prev := viper.Get(vars.ConfigKeyConfigPath)
	viper.Set(vars.ConfigKeyConfigPath, configFile)
	t.Cleanup(func() { viper.Set(vars.ConfigKeyConfigPath, prev) })

	if got, want := replHistoryPath(), filepath.Join(dir, "repl-history"); got != want {
		t.Errorf("replHistoryPath() = %q, want %q", got, want)
	}
}

func TestPrintReplHelp_MentionsKeyConcepts(t *testing.T) {
	// The help text is curated rather than generated, so a regression
	// would be removing a load-bearing line. Pin the most important
	// concepts: the / escape, the exit words, todo-mode defaulting.
	var buf bytes.Buffer
	printReplHelp(&buf)
	out := buf.String()

	for _, want := range []string{
		"todo-mode",
		"/<command>",
		"/interactive, /i",
		"/help",
		"quit",
		"Ctrl-D",
		"add <entry...>",
		"up <index>",
		"down <index>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("help text missing %q\n--- got ---\n%s", want, out)
		}
	}
}
