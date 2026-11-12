package e2e

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTodoAdd_WritesTaskToCurrentContextFile(t *testing.T) {
	e := newEnv(t)

	r := e.mustRun("todo", "add", "buy milk")
	assertContains(t, "stdout", r.stdout, "Added:")
	assertContains(t, "stdout", r.stdout, "buy milk")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), today()+" buy milk\n")
}

func TestTodoAdd_AppendsMultipleTasks(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "first task")
	e.mustRun("todo", "add", "second task")
	e.mustRun("todo", "add", "third task")

	d := today()
	assertEqual(t, "todo.txt", e.readData("todo.txt"),
		d+" first task\n"+d+" second task\n"+d+" third task\n")
}

func TestTodoShow_EmptyContext(t *testing.T) {
	e := newEnv(t)

	r := e.mustRun("todo", "show")
	assertContains(t, "stdout", r.stdout, "Context is empty")
}

func TestTodoShow_ListsTasks(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "alpha")
	e.mustRun("todo", "add", "beta")

	r := e.mustRun("todo", "show")
	assertContains(t, "stdout", r.stdout, "alpha")
	assertContains(t, "stdout", r.stdout, "beta")
	assertContains(t, "stdout", r.stdout, "2 tasks")
}

func TestTodoAddTo_WritesToNamedContext(t *testing.T) {
	e := newEnv(t)

	r := e.mustRun("todo", "add-to", "work", "ship feature")
	assertContains(t, "stdout", r.stdout, "Added:")
	assertContains(t, "stdout", r.stdout, "ship feature")
	assertContains(t, "stdout", r.stdout, "work")
	assertEqual(t, "work.txt", e.readData("work.txt"), today()+" ship feature\n")
	if e.dataExists("todo.txt") {
		got := e.readData("todo.txt")
		if got != "" {
			t.Errorf("todo.txt should be empty or absent, got %q", got)
		}
	}
}

func TestTodoDo_MovesTaskToDone(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "make tea")
	r := e.mustRun("todo", "do", "0")

	assertContains(t, "stdout", r.stdout, "Completed:")
	assertContains(t, "stdout", r.stdout, "make tea")

	assertEqual(t, "todo.txt", e.readData("todo.txt"), "")
	done := e.readData("done.txt")
	if !strings.Contains(done, "make tea") {
		t.Errorf("done.txt should contain completed task, got %q", done)
	}
	if !strings.HasPrefix(done, "x ") {
		t.Errorf("done.txt entry should start with 'x ', got %q", done)
	}
}

func TestTodoDelete_RemovesTask(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "keep this")
	e.mustRun("todo", "add", "delete this")
	r := e.mustRun("todo", "delete", "1")

	assertContains(t, "stdout", r.stdout, "Deleted task:")
	assertContains(t, "stdout", r.stdout, "delete this")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), today()+" keep this\n")
}

func TestTodoMove_BetweenContexts(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "moveme")
	r := e.mustRun("todo", "move", "0", "work")

	assertContains(t, "stdout", r.stdout, "Moved")
	assertContains(t, "stdout", r.stdout, "moveme")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "")
	assertEqual(t, "work.txt", e.readData("work.txt"), today()+" moveme\n")
}

func TestTodoPriority_SetsExplicitPriority(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "urgent thing")
	r := e.mustRun("todo", "priority", "0", "A")

	assertContains(t, "stdout", r.stdout, "After:")
	assertContains(t, "stdout", r.stdout, "(A)")
	assertContains(t, "stdout", r.stdout, "urgent thing")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "(A) "+today()+" urgent thing\n")
}

func TestTodoPriority_KeepsEveryWordOfTheTask(t *testing.T) {
	// A lone "+" used to end the parse and non-ASCII letters to end a
	// word, so rewriting the line for its new priority dropped "B" and
	// split "café".
	e := newEnv(t)
	e.writeData("todo.txt", "fix A + B café @maison note:été\n")

	e.mustRun("todo", "priority", "0", "A")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "(A) fix A + B café @maison note:été\n")

	e.mustRun("todo", "do", "0")
	done := e.readData("done.txt")
	assertContains(t, "done.txt", done, " fix A + B café @maison ")
	assertContains(t, "done.txt", done, " note:été")
}

func TestTodoPriority_BeforeAfterReflectsTransition(t *testing.T) {
	// Locks the "Before:" / "After:" stdout contract: Before should show
	// the pre-mutation state, After the post-mutation state. The naïve
	// rewire to use cases would have collapsed both to the post-mutation
	// state because the domain priority methods mutate the receiver in
	// place. Pin it so we don't regress.
	e := newEnv(t)
	e.mustRun("todo", "add", "task")
	r := e.mustRun("todo", "priority", "0", "A")

	bIdx := strings.Index(r.stdout, "Before:")
	aIdx := strings.Index(r.stdout, "After:")
	if bIdx < 0 || aIdx < 0 || bIdx >= aIdx {
		t.Fatalf("expected Before then After in stdout, got:\n%s", r.stdout)
	}
	beforeLine := r.stdout[bIdx:aIdx]
	afterLine := r.stdout[aIdx:]
	if strings.Contains(beforeLine, "(A)") {
		t.Errorf("Before should NOT contain (A); got %q", beforeLine)
	}
	if !strings.Contains(afterLine, "(A)") {
		t.Errorf("After should contain (A); got %q", afterLine)
	}
}

func TestTodoPriorityIncrease_LowersLetter(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "(C) something")
	e.mustRun("todo", "+", "0")

	// rebuild's todo.txt-compliant ordering: "(B) <date> something".
	assertContains(t, "todo.txt", e.readData("todo.txt"), "(B) "+today()+" something")
}

// NOTE: `htt todo - <id>` is not reachable via subprocess args because
// Cobra resolves "-" as a flag prefix before any "--" separator. This is a
// pre-existing CLI bug; the `+` test above covers the symmetric priority-
// change code path. Revisit when the CLI layer is refactored.

func TestTodoReplace_OverwritesTask(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "old text")
	r := e.mustRun("todo", "replace", "0", "new", "text")

	assertContains(t, "stdout", r.stdout, "After:")
	assertContains(t, "stdout", r.stdout, "new text")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "new text\n")
}

func TestTodoSearch_MatchesByRegex(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "buy bread")
	e.mustRun("todo", "add", "buy milk")
	e.mustRun("todo", "add", "call alice")

	r := e.mustRun("todo", "search", "buy")
	assertContains(t, "stdout", r.stdout, "buy bread")
	assertContains(t, "stdout", r.stdout, "buy milk")
	if strings.Contains(r.stdout, "call alice") {
		t.Errorf("search ^buy should not include 'call alice', got:\n%s", r.stdout)
	}
	assertContains(t, "stdout", r.stdout, "2 out of 3")
}

func TestTodoShow_SortKeepsOriginalIndices(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "--due", "2030-01-03", "alpha")
	e.mustRun("todo", "add", "beta")
	e.mustRun("todo", "add", "--due", "2030-01-01", "gamma")

	d := today()
	r := e.mustRun("todo", "show", "--sort", "due")
	assertInOrder(t, "show --sort due", r.stdout,
		"  2 "+d+" gamma", "  0 "+d+" alpha", "  1 "+d+" beta")
	assertContains(t, "stdout", r.stdout, "3 of 3 tasks shown.")

	// Reversed, the undated task still comes last.
	r = e.mustRun("todo", "show", "--sort", "-due")
	assertInOrder(t, "show --sort -due", r.stdout,
		"  0 "+d+" alpha", "  2 "+d+" gamma", "  1 "+d+" beta")

	// The index printed beside gamma is the one delete acts on.
	e.mustRun("todo", "delete", "2")
	assertEqual(t, "todo.txt", e.readData("todo.txt"),
		d+" alpha due:2030-01-03\n"+d+" beta\n")
}

func TestTodoShow_FiltersCombineAndCountTheView(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "buy milk @home")
	e.mustRun("todo", "add", "--due", "2030-01-01", "fix the sink @home")
	e.mustRun("todo", "add", "write report")

	r := e.mustRun("todo", "show", "--filter", "has-tag:home", "--filter", "no-due")
	assertContains(t, "stdout", r.stdout, "  0 "+today()+" buy milk @home")
	if strings.Contains(r.stdout, "sink") || strings.Contains(r.stdout, "report") {
		t.Errorf("filters should leave only 'buy milk', got:\n%s", r.stdout)
	}
	assertContains(t, "stdout", r.stdout, "1 of 3 tasks shown.")

	r = e.mustRun("todo", "show", "--filter", "overdue")
	assertContains(t, "stdout", r.stdout, "No tasks matched query.")
}

func TestTodoShow_IndicesSurviveBlankLines(t *testing.T) {
	// A hand-edited file can carry blank lines. The index show prints
	// must still be the one delete acts on.
	e := newEnv(t)
	e.writeData("todo.txt", "alpha\n\nbeta\ngamma\n")

	r := e.mustRun("todo", "show")
	assertInOrder(t, "show", r.stdout, "  0 alpha", "  1 beta", "  2 gamma")

	e.mustRun("todo", "delete", "1")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "alpha\ngamma\n")
}

func TestTodoShow_UnknownFilterFails(t *testing.T) {
	e := newEnv(t)
	e.mustRun("todo", "add", "alpha")

	r := e.run("todo", "show", "--filter", "someday")
	if r.exitCode == 0 {
		t.Errorf("expected non-zero exit for an unknown filter, got 0")
	}
	assertContains(t, "stderr", r.stderr, `invalid --filter "someday"`)
}

func TestTodoContext_ShowsCurrent(t *testing.T) {
	e := newEnv(t)

	r := e.mustRun("todo", "context")
	assertContains(t, "stdout", r.stdout, "todo")
}

func TestTodoContext_SwitchesAndPersists(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "seed")

	r := e.mustRun("todo", "context", "work")
	assertContains(t, "stdout", r.stdout, "Now using context: work")

	e.mustRun("todo", "add", "task in work")
	assertEqual(t, "work.txt", e.readData("work.txt"), today()+" task in work\n")

	r2 := e.mustRun("todo", "context")
	assertContains(t, "stdout", r2.stdout, "work")
}

func TestTodoStatus_ListsContextsAndCurrent(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "first")
	e.mustRun("todo", "add-to", "work", "ship feature")

	r := e.mustRun("todo", "status")
	assertContains(t, "stdout", r.stdout, "Available Contexts")
	assertContains(t, "stdout", r.stdout, "todo")
	assertContains(t, "stdout", r.stdout, "work")
	assertContains(t, "stdout", r.stdout, "Current Context")
	assertContains(t, "stdout", r.stdout, "first")
}

func TestTodoDelete_OutOfRangeFails(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "only task")
	r := e.run("todo", "delete", "99")

	if r.exitCode == 0 {
		t.Errorf("expected non-zero exit, got 0\nstdout:\n%s\nstderr:\n%s", r.stdout, r.stderr)
	}
	assertEqual(t, "todo.txt", e.readData("todo.txt"), today()+" only task\n")
}

func TestTodoPriority_InvalidPriorityFails(t *testing.T) {
	e := newEnv(t)

	e.mustRun("todo", "add", "thing")
	r := e.run("todo", "priority", "0", "Z")

	if r.exitCode == 0 {
		t.Errorf("expected non-zero exit, got 0\nstdout:\n%s\nstderr:\n%s", r.stdout, r.stderr)
	}
}

func TestErrorPath_StderrCarriesPrefixAndExitCode(t *testing.T) {
	// Step 12 contract: command errors propagate up RunE, are caught by
	// main.go's Execute() wrapper, printed to stderr with "❌" prefix,
	// and the binary exits non-zero. Pin the contract end-to-end.
	e := newEnv(t)
	r := e.run("todo", "delete", "999")

	if r.exitCode == 0 {
		t.Errorf("expected non-zero exit, got 0")
	}
	if !strings.Contains(r.stderr, "❌") {
		t.Errorf("stderr should carry ❌ prefix from main.go wrapper, got: %q", r.stderr)
	}
	if r.stdout != "" {
		t.Errorf("error should not pollute stdout, got: %q", r.stdout)
	}
}

func TestInteractive_LogFlagsAreCheckedBeforeTheTUIStarts(t *testing.T) {
	// Each of these fails before the TUI takes over the terminal, so
	// they run fine without a TTY.
	e := newEnv(t)
	logPath := filepath.Join(e.home, "tui.log")

	cases := []struct {
		args []string
		want string
	}{
		{[]string{"interactive", "--log-level", "warn"}, "--log-level needs --log-file"},
		{[]string{"interactive", "--log-file", logPath, "--log-level", "loud"}, `invalid --log-level "loud"`},
		{[]string{"interactive", "--log-file", filepath.Join(e.home, "missing", "tui.log")}, "open --log-file"},
	}
	for _, tc := range cases {
		r := e.run(tc.args...)
		if r.exitCode == 0 {
			t.Errorf("htt %s: expected non-zero exit", strings.Join(tc.args, " "))
		}
		assertContains(t, "stderr", r.stderr, tc.want)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Errorf("rejected flags should not create %s (stat err = %v)", logPath, err)
	}
}

func TestInteractive_HelpStatesTheDebugDefault(t *testing.T) {
	e := newEnv(t)
	r := e.mustRun("interactive", "--help")
	assertContains(t, "help", r.stdout, "logs at debug level unless --log-level is set")
	assertContains(t, "help", r.stdout, `(default "debug")`)
	if strings.Contains(r.stdout, "--debug") {
		t.Errorf("--debug was replaced by --log-file and should be gone:\n%s", r.stdout)
	}
}

func TestInteractive_BadThemeConfigFailsBeforeTheTUIStarts(t *testing.T) {
	cases := []struct {
		name   string
		config string
		want   []string
	}{
		{"unknown theme", "tui:\n  theme: solarised\n", []string{`unknown theme "solarised"`, "solarized-dark"}},
		{"unknown colour role", "tui:\n  colors:\n    cursor: \"#ffffff\"\n", []string{`unknown colour "cursor"`}},
		// Unquoted, YAML reads "#f92672" as a comment and the value is empty.
		{"unquoted hex", "tui:\n  colors:\n    accent: #f92672\n", []string{"tui.colors.accent", "quote hex colours"}},
		{"colour name", "tui:\n  colors:\n    accent: red\n", []string{`invalid colour "red"`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.writeConfig(tc.config)
			r := e.run("interactive")
			if r.exitCode == 0 {
				t.Errorf("expected a non-zero exit")
			}
			for _, want := range tc.want {
				assertContains(t, "stderr", r.stderr, want)
			}
		})
	}
}

func TestInteractive_ThemeConfigLeavesOtherCommandsAlone(t *testing.T) {
	e := newEnv(t)
	e.writeConfig("tui:\n  theme: monokai\n  colors:\n    accent: \"#abcdef\"\n")
	e.mustRun("todo", "add", "still works")
	r := e.mustRun("config", "yaml")
	assertContains(t, "config yaml", r.stdout, "theme: monokai")
}

func TestVersion_PrintsTheBuildVersion(t *testing.T) {
	e := newEnv(t)
	r := e.mustRun("--version")
	// The test binary is built from the checkout, so Go stamps a
	// pseudo-version from git.
	assertContains(t, "stdout", r.stdout, "htt version v")
}

func TestUpdateCheck_NoNetworkWithoutATerminal(t *testing.T) {
	// The e2e harness runs htt without a terminal, as a script would.
	// Route any HTTPS through a local "proxy" that only counts
	// connection attempts: there must be none, even with the check on
	// (the default) and no record of a previous check.
	proxy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var attempts atomic.Int32
	go func() {
		for {
			conn, err := proxy.Accept()
			if err != nil {
				return
			}
			attempts.Add(1)
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() { _ = proxy.Close() })

	e := newEnv(t)
	e.extraEnv = []string{"HTTPS_PROXY=http://" + proxy.Addr().String(), "HTTP_PROXY=http://" + proxy.Addr().String()}
	e.mustRun("todo", "add", "offline task")
	e.mustRun("todo", "show")
	e.mustRun("status")
	e.mustRun("report")

	// The check is on by default, under the name the README documents.
	assertContains(t, "config yaml", e.mustRun("config", "yaml").stdout, "check_for_updates: true")

	time.Sleep(200 * time.Millisecond) // let any stray connection land
	if n := attempts.Load(); n != 0 {
		t.Errorf("htt opened %d connection(s) without a terminal; want none", n)
	}
	if _, err := os.Stat(filepath.Join(e.home, ".cache", "htt", "update-check.json")); !os.IsNotExist(err) {
		t.Errorf("no check should have been recorded (stat err = %v)", err)
	}
}

func TestInteractive_HelpListsThemesAndColours(t *testing.T) {
	e := newEnv(t)
	r := e.mustRun("interactive", "--help")
	assertContains(t, "help", r.stdout, "Themes: default, dracula, gruvbox, monokai, nord")
	assertContains(t, "help", r.stdout, "Colours: foreground, background, selection, accent, muted, help, separator")
}
