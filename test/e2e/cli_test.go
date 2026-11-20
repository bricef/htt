package e2e

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
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

func TestTodoPriority_KeepsTimesAndLinksInPlace(t *testing.T) {
	// 10:30 and the link used to read as key:value annotations, which a
	// rewrite moves to the end of the line.
	e := newEnv(t)
	e.writeData("todo.txt", "call Bob at 10:30 about https://example.com/report today\n")

	e.mustRun("todo", "priority", "0", "A")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "(A) call Bob at 10:30 about https://example.com/report today\n")
}

func TestTodoPriorityAndDo_KeepAnnotationsWhereTheyAre(t *testing.T) {
	// Rewriting a task used to move its annotations to the end of the
	// line, in random order, keeping only the last of a repeated key.
	e := newEnv(t)
	e.writeData("todo.txt", "pay due:2026-06-01 rent ref:42 a:1 then a:2 via:bank\n")

	e.mustRun("todo", "priority", "0", "A")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "(A) pay due:2026-06-01 rent ref:42 a:1 then a:2 via:bank\n")

	e.mustRun("todo", "do", "0")
	assertEqual(t, "done.txt", e.readData("done.txt"),
		"x (A) "+today()+" pay due:2026-06-01 rent ref:42 a:1 then a:2 via:bank context:todo\n")
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

func TestConfig_KeysFromTheRemovedSyncCommandStillLoad(t *testing.T) {
	// Config files written before `htt sync` was removed carry its
	// keys, and the default file htt wrote on first run had most of
	// them. htt ignores them now; they must not stop it starting.
	e := newEnv(t)
	e.writeConfig("backing_repo_url: git@github.com:you/tasks.git\n" +
		"ssh_key_file: /home/you/.ssh/id_ed25519\n" +
		"file_match_patterns:\n- '*.txt'\n- .context\n- timelogs\n- .gitignore\n" +
		"remote_name: origin\n" +
		"author_name: You\n" +
		"author_email: you@example.com\n")

	e.mustRun("todo", "add", "still works")
	assertContains(t, "todo show", e.mustRun("todo", "show").stdout, "still works")
	e.mustRun("status")
	e.mustRun("report")
	e.mustRun("config", "yaml")
}

func TestDataDir_AGitRepositoryInsideIsLeftAlone(t *testing.T) {
	// People keep the data directory in git, so it can hold .git/ and
	// .gitignore. Neither is a context, and htt never writes to them.
	e := newEnv(t)
	e.writeData(".git/HEAD", "ref: refs/heads/master\n")
	e.writeData(".gitignore", "*.bak\n")
	e.writeData("todo.txt", "first\n")

	e.mustRun("todo", "add-to", "work", "ship feature")
	e.mustRun("log", "start")
	e.mustRun("log", "end")

	r := e.mustRun("todo", "status")
	assertContains(t, "stdout", r.stdout, "Available Contexts: todo work \n")
	assertEqual(t, ".git/HEAD", e.readData(".git/HEAD"), "ref: refs/heads/master\n")
	assertEqual(t, ".gitignore", e.readData(".gitignore"), "*.bak\n")
}

func TestVersion_PrintsTheBuildVersion(t *testing.T) {
	e := newEnv(t)
	r := e.mustRun("--version")
	// The test binary is built from the checkout, so Go stamps a
	// pseudo-version from git.
	assertContains(t, "stdout", r.stdout, "htt version v")
}

func TestFirstRun_SaysItCreatedTheConfigOnStderr(t *testing.T) {
	// On a first run htt writes a default config file and says so. That
	// note goes to stderr: on stdout it got in front of a command's own
	// output, so `htt --version` on a fresh machine printed three lines.
	e := newEnv(t)
	configFile := filepath.Join(e.home, ".config", "htt", "config.yaml")
	if err := os.Remove(configFile); err != nil {
		t.Fatal(err)
	}

	r := e.mustRun("--version")
	if lines := strings.Split(strings.TrimSpace(r.stdout), "\n"); len(lines) != 1 || !strings.HasPrefix(lines[0], "htt version v") {
		t.Errorf("stdout should be the version line alone, got %q", r.stdout)
	}
	assertContains(t, "stderr", r.stderr, "Creating a default file at "+configFile)
	if _, err := os.Stat(configFile); err != nil {
		t.Errorf("the first run should still create the config file: %v", err)
	}
}

func TestFirstRun_MakesTheConfigDirectory(t *testing.T) {
	// Where htt has never run, there's no config directory either. htt
	// found the directory to make with path.Dir, which splits only at /,
	// so on Windows it made none and couldn't write the config file.
	e := newEnv(t)
	if err := os.RemoveAll(filepath.Join(e.home, ".config")); err != nil {
		t.Fatal(err)
	}

	r := e.mustRun("todo", "add", "first task")
	assertEqual(t, "stdout", r.stdout, "Added: "+today()+" first task\n")
	if _, err := os.Stat(filepath.Join(e.home, ".config", "htt", "config.yaml")); err != nil {
		t.Errorf("the first run should create the config file: %v", err)
	}
	assertEqual(t, "todo.txt", e.readData("todo.txt"), today()+" first task\n")
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
	if _, err := os.Stat(filepath.Join(e.cacheDir(), "htt", "update-check.json")); !os.IsNotExist(err) {
		t.Errorf("no check should have been recorded (stat err = %v)", err)
	}
}

func TestInteractive_HelpListsThemesAndColours(t *testing.T) {
	e := newEnv(t)
	r := e.mustRun("interactive", "--help")
	assertContains(t, "help", r.stdout, "Themes: default, dracula, gruvbox, monokai, nord")
	assertContains(t, "help", r.stdout, "Colours: foreground, background, selection, accent, muted, help,")
	for _, role := range []string{"separator,", "priority_a,", "plus_tag,", "at_tag,", "hash_tag,", "annotation,", "date."} {
		assertContains(t, "help", r.stdout, role)
	}
}

func TestReport_DeletedTasksAreNotCountedAsAdded(t *testing.T) {
	// The Added section used to walk the archive like any other
	// context, so a task added and deleted in the period was counted
	// twice: under Deleted, and under Added in a context named
	// "archive". A task added and completed still counts as added,
	// under the context it was completed from.
	e := newEnv(t)
	e.mustRun("todo", "add", "keep me")
	e.mustRun("todo", "add", "finish me")
	e.mustRun("todo", "add", "drop me")
	e.mustRun("todo", "do", "1")
	e.mustRun("todo", "delete", "1")

	r := e.mustRun("report")
	addedAt := strings.Index(r.stdout, "Added (")
	deletedAt := strings.Index(r.stdout, "Deleted (")
	if addedAt < 0 || deletedAt < addedAt {
		t.Fatalf("expected an Added section before a Deleted section, got:\n%s", r.stdout)
	}
	added, deleted := r.stdout[addedAt:deletedAt], r.stdout[deletedAt:]

	assertInOrder(t, "Added", added, "Added (2)\n", "  todo:\n", "    finish me", "    keep me")
	if strings.Contains(added, "drop me") || strings.Contains(added, "archive") {
		t.Errorf("Added should leave out the deleted task, got:\n%s", added)
	}
	assertInOrder(t, "Deleted", deleted, "Deleted (1)\n", "  todo:\n", "    drop me")
}

func TestRepl_HistoryPersistsAcrossSessions(t *testing.T) {
	// The REPL built its history path from config_path, which is the
	// config file, so creating "config.yaml/" as a directory failed and
	// history was switched off. It lives next to config.yaml.
	e := newEnv(t)
	e.mustRunWithInput("add first task\nq\n", "repl")
	e.mustRunWithInput("add second task\nq\n", "repl")

	b, err := os.ReadFile(filepath.Join(e.home, ".config", "htt", "repl-history"))
	if err != nil {
		t.Fatalf("no REPL history next to config.yaml: %v", err)
	}
	assertInOrder(t, "repl-history", string(b), "add first task\n", "add second task\n")
}

func TestRepl_PipedInputIsReadAsItIs(t *testing.T) {
	// Lines from a pipe are read with no prompt or echo, so the only
	// prompt in the output is the one htt prints above a command's
	// output. readline does this by itself on Unix only. On Windows it
	// took stdin for the console: it read nothing from a pipe, and its
	// terminal mode draws the prompt again for every character read.
	e := newEnv(t)
	r := e.mustRunWithInput("add buy milk\nq\n", "repl")
	if n := strings.Count(r.stdout, "htt(todo)> "); n != 1 {
		t.Errorf("want the prompt once, above the add's output; got it %d times:\n%q", n, r.stdout)
	}
	assertEqual(t, "todo.txt", e.readData("todo.txt"), today()+" buy milk\n")
}

func TestRepl_TheTUINeedsATerminal(t *testing.T) {
	// The REPL hands its terminal to the TUI; reading a pipe, it has
	// none to hand over, so /interactive and its alias say so, and the
	// REPL carries on.
	e := newEnv(t)
	r := e.mustRunWithInput("/interactive\n/i\nadd still here\nq\n", "repl")
	if n := strings.Count(r.stderr, "the TUI needs a terminal"); n != 2 {
		t.Errorf("want the TUI to need a terminal twice, for /interactive and /i; got %d:\n%s", n, r.stderr)
	}
	assertEqual(t, "todo.txt", e.readData("todo.txt"), today()+" still here\n")
}

func TestRepl_RefusesANestedREPLAndTheGUI(t *testing.T) {
	// A REPL inside the REPL, and the GUI, which serves until a Ctrl-C
	// that would end the REPL too, aren't run.
	e := newEnv(t)
	e.writeConfig("check_for_updates: false\n")
	r := e.mustRunWithInput("/repl\n/gui --no-open\nadd still here\nq\n", "repl")
	assertInOrder(t, "stderr", r.stderr,
		`"repl" is not available inside the REPL`, `"gui" is not available inside the REPL`)
	assertEqual(t, "todo.txt", e.readData("todo.txt"), today()+" still here\n")
}

// replScreens splits REPL output at each clear-screen sequence: the
// last element is what the terminal shows when the REPL exits.
func replScreens(t *testing.T, stdout string) []string {
	t.Helper()
	screens := strings.Split(stdout, "\x1b[H\x1b[2J")
	if len(screens) < 2 {
		t.Fatalf("the REPL never cleared the screen:\n%q", stdout)
	}
	return screens[1:]
}

func TestRepl_CommandOutputStaysOnScreen(t *testing.T) {
	// Each cycle cleared the screen straight after a command ran, so
	// its output (Added: …, a report) was wiped at once. The screen is
	// now cleared before a command runs: after it come the command as
	// typed, its output, and the view, refreshed to show its effect.
	e := newEnv(t)
	d := today()
	r := e.mustRunWithInput("add buy milk\n/report\nq\n", "repl")
	screens := replScreens(t, r.stdout)
	if len(screens) != 3 {
		t.Fatalf("want 3 screens (start, add, /report), got %d:\n%q", len(screens), r.stdout)
	}

	assertInOrder(t, "start", screens[0], "(todo): Context is empty.\n")
	assertInOrder(t, "after add", screens[1],
		"htt(todo)> add buy milk\n", "Added: "+d+" buy milk\n", "  0 "+d+" buy milk\n", "(todo): 1 tasks\n")
	assertInOrder(t, "after /report", screens[2],
		"htt(todo)> /report\n", "Activity since", "Added (1)\n", "  0 "+d+" buy milk\n", "(todo): 1 tasks\n")
	// The clear still wipes the previous command's output.
	if strings.Contains(screens[2], "Added: ") {
		t.Errorf("the add's output should be gone after /report:\n%s", screens[2])
	}
}

func TestLog_EveryCommandThatLogsConfirmsItsEntry(t *testing.T) {
	// log start, log end and workon stopped printing anything in the
	// 2026-05-15 timelog refactor; only log add still confirmed its
	// entry.
	e := newEnv(t)
	e.mustRun("todo", "add", "write the report +work")

	assertEqual(t, "log start", e.mustRun("log", "start").stdout, "Logging entry: @start\n")
	assertEqual(t, "log add", e.mustRun("log", "add", "reading", "mail").stdout, "Logging entry: reading mail\n")
	assertEqual(t, "workon", e.mustRun("workon", "0").stdout, "Logging entry: "+today()+" write the report +work\n")
	assertEqual(t, "log end", e.mustRun("log", "end").stdout, "Logging entry: @end\n")
}

func TestEditCommands_EditorProblemsFailLikeTodoEdit(t *testing.T) {
	// todo edit-done, todo edit-archive and log edit ended through
	// utils.Fatal: a timestamped log line instead of the ❌ error
	// todo edit returns. All four now fail the same way.
	commands := [][]string{
		{"todo", "edit", "0"},
		{"todo", "edit-done"},
		{"todo", "edit-archive"},
		{"log", "edit"},
	}
	// Windows has no false, so its failing editor is cmd's exit there.
	// And an unset TERM isn't dumb on Windows, whose consoles leave it
	// unset, so $VISUAL runs, and fails to start.
	fails, failsWithArgs := "false", "false --wait"
	unsetTerm := "❌ $EDITOR is not set ($VISUAL isn't used when TERM is dumb or unset)\n"
	if runtime.GOOS == "windows" {
		fails, failsWithArgs = "cmd /c exit 1", "cmd /c exit 1 --wait"
		unsetTerm = "❌ start editor: exec: \"no-such-visual-editor\": executable file not found in %PATH%\n"
	}
	cases := []struct {
		name   string
		env    []string // the editor variables set; nil sets none
		stderr string
	}{
		{"no editor", []string{"TERM=xterm"}, "❌ neither $VISUAL nor $EDITOR is set\n"},
		{"empty VISUAL and EDITOR", []string{"TERM=xterm", "VISUAL=", "EDITOR="}, "❌ neither $VISUAL nor $EDITOR is set\n"},
		// Not a real editor: if VISUAL ran, it would fail at once
		// rather than wait for a terminal.
		{"only VISUAL on a dumb terminal", []string{"TERM=dumb", "VISUAL=no-such-visual-editor"}, "❌ $EDITOR is not set ($VISUAL isn't used when TERM is dumb or unset)\n"},
		// As git does, an unset TERM is a dumb one (but on Windows).
		{"only VISUAL, TERM unset", []string{"VISUAL=no-such-visual-editor"}, unsetTerm},
		{"editor fails", []string{"EDITOR=" + fails}, "❌ run editor: exit status 1\n"},
		{"editor with arguments fails", []string{"EDITOR=" + failsWithArgs}, "❌ run editor: exit status 1\n"},
	}
	for _, tc := range cases {
		for _, args := range commands {
			label := tc.name + ": htt " + strings.Join(args, " ")
			e := newEnv(t)
			e.mustRun("todo", "add", "a task")
			e.extraEnv = tc.env
			r := e.run(args...)
			if r.exitCode != 1 {
				t.Errorf("%s: exit code %d, want 1", label, r.exitCode)
			}
			assertEqual(t, label+": stderr", r.stderr, tc.stderr)
			assertEqual(t, label+": stdout", r.stdout, "")
		}
	}
}

func TestEditCommands_SaveWhatTheEditorWrites(t *testing.T) {
	skipOnWindowsForShellEditor(t)
	e := newEnv(t)
	editor := filepath.Join(e.home, "editor.sh")
	if err := os.WriteFile(editor, []byte("#!/bin/sh\necho 'edited by the editor' > \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.extraEnv = []string{"EDITOR=" + editor}
	e.mustRun("todo", "add", "a task")

	r := e.mustRun("todo", "edit", "0")
	assertContains(t, "stdout", r.stdout, "After:  edited by the editor")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "edited by the editor\n")

	e.mustRun("todo", "edit-done")
	assertEqual(t, "done.txt", e.readData("done.txt"), "edited by the editor\n")
}

func TestConfigDir_PrintsTheConfigDirectory(t *testing.T) {
	// `htt config dir` printed the tracker directory (where the
	// current-context pointer lives), not the directory its name says.
	e := newEnv(t)
	configDir := filepath.Join(e.home, ".config", "htt")
	assertEqual(t, "config dir", e.mustRun("config", "dir").stdout, configDir+"\n")

	// Moving the data and tracker directories doesn't move it.
	e.writeConfig("data_path: " + filepath.Join(e.home, "tasks") + "\ntracker_path: " + filepath.Join(e.home, "tracker") + "\n")
	assertEqual(t, "config dir with moved data", e.mustRun("config", "dir").stdout, configDir+"\n")

	// A config.yaml in the working directory is the config in effect.
	e.workDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(e.workDir, "config.yaml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "config dir with a local config.yaml", e.mustRun("config", "dir").stdout, e.workDir+"\n")

	assertContains(t, "config dir --help", e.mustRun("config", "dir", "--help").stdout, "config directory")
}

func TestRepl_AnEditorProblemDoesNotEndTheSession(t *testing.T) {
	// utils.Fatal exited the process, taking the REPL with it.
	e := newEnv(t)
	e.extraEnv = []string{"TERM=xterm"}
	r := e.mustRunWithInput("edit-done\nadd still here\nq\n", "repl")
	assertContains(t, "stderr", r.stderr, "neither $VISUAL nor $EDITOR is set")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), today()+" still here\n")
}

// A line with a date htt can't read: "All lines are valid. If we
// detect a misformatted or incorrect date, we should warn, but not
// break, crash or error out." The line loads with the date read as
// text, and every command works as before; a warning naming the file
// and line goes to stderr, once per line per run.

// badDatesTodo is a todo.txt with three bad dates. The blank line
// makes file lines differ from task numbers: the warnings name lines.
const badDatesTodo = "fine task\n\n2026-13-45 call the bank\nmeet on 2026-5-3\npay rent due:friday\n"

func badDateWarnings(path string, lines ...int) string {
	messages := map[int]string{
		3: "2026-13-45 isn't a real date, so it's text",
		4: "2026-5-3 isn't written YYYY-MM-DD (2026-05-03), so it's text",
		5: "due:friday isn't a YYYY-MM-DD date, so the task has no due date",
	}
	out := ""
	for _, line := range lines {
		out += fmt.Sprintf("warning: %s:%d: %s\n", path, line, messages[line])
	}
	return out
}

func TestDates_BadDatesLoadAndWarnOnStderr(t *testing.T) {
	e := newEnv(t)
	e.writeData("todo.txt", badDatesTodo)
	warnings := badDateWarnings(filepath.Join(e.dataDir, "todo.txt"), 3, 4, 5)

	r := e.mustRun("todo", "show")
	assertEqual(t, "todo show: stderr", r.stderr, warnings)
	assertEqual(t, "todo show: stdout", r.stdout,
		"\n  0 fine task\n  1 2026-13-45 call the bank\n  2 meet on 2026-5-3\n  3 pay rent due:friday\n\n(todo): 4 tasks\n")

	for _, args := range [][]string{
		{"todo", "show", "--filter", "no-due", "--sort", "due"},
		{"todo", "search", "bank"},
		{"todo", "status"},
		{"status"},
		{"report"},
		{"workon", "1"},
	} {
		label := "htt " + strings.Join(args, " ")
		r := e.mustRun(args...) // exit 0
		assertEqual(t, label+": stderr", r.stderr, warnings)
		if strings.Contains(r.stdout, "warning") || strings.Contains(r.stdout, "real date") {
			t.Errorf("%s: a warning reached stdout:\n%s", label, r.stdout)
		}
	}
	assertContains(t, "show --filter no-due", e.mustRun("todo", "show", "--filter", "no-due").stdout, "4 of 4 tasks shown.")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), badDatesTodo) // nothing rewrote it

	// Commands that don't read the tasks say nothing.
	for _, args := range [][]string{{"config", "dir"}, {"todo", "edit-done", "--help"}} {
		assertEqual(t, "htt "+strings.Join(args, " ")+": stderr", e.mustRun(args...).stderr, "")
	}
	// log show reads the time log, where workon put the bank's line.
	assertEqual(t, "htt log show: stderr", e.mustRun("log", "show").stderr,
		"warning: "+filepath.Join(e.dataDir, "timelogs", today()+".log")+":1: 2026-13-45 isn't a real date, so it's text\n")
}

func TestDates_CommandsThatChangeTasksStillWork(t *testing.T) {
	e := newEnv(t)
	e.writeData("todo.txt", badDatesTodo)
	todo := filepath.Join(e.dataDir, "todo.txt")
	d := today()

	r := e.mustRun("todo", "priority", "1", "A")
	assertEqual(t, "priority: stderr", r.stderr, badDateWarnings(todo, 3, 4, 5))
	assertContains(t, "priority: stdout", r.stdout, "After:  (A) 2026-13-45 call the bank")
	// The priority sorts the list, and saving drops the blank line.
	assertEqual(t, "todo.txt", e.readData("todo.txt"),
		"(A) 2026-13-45 call the bank\nfine task\nmeet on 2026-5-3\npay rent due:friday\n")

	e.mustRun("todo", "add", "new task")
	e.mustRun("todo", "do", "0")     // the bank: done.txt now has a bad date
	e.mustRun("todo", "delete", "1") // 2026-5-3: the archive has one
	assertEqual(t, "done.txt", e.readData("done.txt"), "x (A) "+d+" 2026-13-45 call the bank context:todo\n")
	assertContains(t, "archive.txt", e.readData("archive.txt"), "meet on 2026-5-3 archived-from:todo")

	// Completing reads the current list, not done.txt it appends to.
	r = e.mustRun("todo", "do", "0")
	assertEqual(t, "do: stderr", r.stderr, "warning: "+todo+":2: "+
		"due:friday isn't a YYYY-MM-DD date, so the task has no due date\n")

	// report reads done.txt twice and warns once; the archive (deleted
	// tasks, whose dates nothing reads) never warns.
	r = e.mustRun("report")
	assertEqual(t, "report: stderr", r.stderr,
		"warning: "+filepath.Join(e.dataDir, "done.txt")+":1: 2026-13-45 isn't a real date, so it's text\n"+
			"warning: "+todo+":1: due:friday isn't a YYYY-MM-DD date, so the task has no due date\n")
	assertInOrder(t, "report", r.stdout, "Completed (2)\n", "Added (1)\n", "Deleted (1)\n", "meet on 2026-5-3")
}

func TestRepl_DateWarningsOncePerSession(t *testing.T) {
	e := newEnv(t)
	e.writeData("todo.txt", badDatesTodo)
	r := e.mustRunWithInput("show\n/report\nshow\nq\n", "repl")
	assertEqual(t, "stderr", r.stderr, badDateWarnings(filepath.Join(e.dataDir, "todo.txt"), 3, 4, 5))
}

// A task typed with a date htt can't read warns as it's written, not
// only when a later command reads the file. The warning names the line
// the task was written to; the task is written all the same, and stdout
// and the exit code are the command's.
func TestDates_TypedTasksWarnAsTheyAreWritten(t *testing.T) {
	skipOnWindowsForShellEditor(t)
	e := newEnv(t)
	// Saving drops the blank line: the warning names the line as saved.
	e.writeData("todo.txt", "fine task\n\nother task\n")
	todo := filepath.Join(e.dataDir, "todo.txt")
	d := today()

	r := e.mustRun("todo", "add", "pay", "rent", "2026-13-45")
	assertEqual(t, "add: stderr", r.stderr, "warning: "+todo+":3: 2026-13-45 isn't a real date, so it's text\n")
	assertEqual(t, "add: stdout", r.stdout, "Added: "+d+" pay rent 2026-13-45\n")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "fine task\nother task\n"+d+" pay rent 2026-13-45\n")

	r = e.mustRun("todo", "add-to", "work", "meet on 2026-5-3")
	assertEqual(t, "add-to: stderr", r.stderr, "warning: "+filepath.Join(e.dataDir, "work.txt")+
		":1: 2026-5-3 isn't written YYYY-MM-DD (2026-05-03), so it's text\n")
	assertEqual(t, "work.txt", e.readData("work.txt"), d+" meet on 2026-5-3\n")

	// replace reads the list, which warns about line 3 (this is a new
	// run), then warns about the replacement.
	r = e.mustRun("todo", "replace", "0", "call", "due:friday")
	assertEqual(t, "replace: stderr", r.stderr,
		"warning: "+todo+":3: 2026-13-45 isn't a real date, so it's text\n"+
			"warning: "+todo+":1: due:friday isn't a YYYY-MM-DD date, so the task has no due date\n")
	assertEqual(t, "replace: stdout", r.stdout, "Before: fine task\nAfter: call due:friday\n")

	editor := filepath.Join(e.home, "editor.sh")
	if err := os.WriteFile(editor, []byte("#!/bin/sh\necho 'other task on 2026-02-30' > \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.extraEnv = []string{"EDITOR=" + editor}
	r = e.mustRun("todo", "edit", "1")
	assertEqual(t, "edit: stderr", r.stderr,
		"warning: "+todo+":1: due:friday isn't a YYYY-MM-DD date, so the task has no due date\n"+
			"warning: "+todo+":3: 2026-13-45 isn't a real date, so it's text\n"+
			"warning: "+todo+":2: 2026-02-30 isn't a real date, so it's text\n")
	assertEqual(t, "edit: stdout", r.stdout, "Before: other task\nAfter:  other task on 2026-02-30\n")
	assertEqual(t, "todo.txt after edit", e.readData("todo.txt"),
		"call due:friday\nother task on 2026-02-30\n"+d+" pay rent 2026-13-45\n")

	r = e.mustRun("log", "add", "read", "the", "2026-5-3", "minutes")
	assertEqual(t, "log add: stderr", r.stderr, "warning: "+filepath.Join(e.dataDir, "timelogs", d+".log")+
		":1: 2026-5-3 isn't written YYYY-MM-DD (2026-05-03), so it's text\n")
	assertEqual(t, "log add: stdout", r.stdout, "Logging entry: read the 2026-5-3 minutes\n")

	// Fine dates say nothing.
	r = e.mustRun("todo", "add-to", "home", "renew passport due:2026-12-01")
	assertEqual(t, "a fine task: stderr", r.stderr, "")
}

// Time-log entries are tasks too: a command that reads a time log warns
// about the entries whose dates htt can't read, as it does for task
// files. The ts: timestamp htt gives each entry is its own, and never
// warns.
func TestDates_TimeLogEntriesWarnOnStderr(t *testing.T) {
	e := newEnv(t)
	d := today()
	logRel := filepath.Join("timelogs", d+".log")
	logFile := filepath.Join(e.dataDir, logRel)
	// The blank line makes file lines differ from entry numbers.
	e.writeData(logRel, "@start ts:"+d+"T00:01:00Z\n\n"+
		"call the bank about 2026-13-45 ts:"+d+"T00:02:00Z\n"+
		"review due:friday ts:"+d+"T00:03:00Z\n")
	warnings := func(lines ...int) string {
		messages := map[int]string{
			3: "2026-13-45 isn't a real date, so it's text",
			4: "due:friday isn't a YYYY-MM-DD date, so the task has no due date",
		}
		out := ""
		for _, line := range lines {
			out += fmt.Sprintf("warning: %s:%d: %s\n", logFile, line, messages[line])
		}
		return out
	}

	for _, args := range [][]string{
		{"log", "show"},
		{"log", "status"},
		{"log", "active"},
		{"status"},
		{"report"},
	} {
		label := "htt " + strings.Join(args, " ")
		r := e.mustRun(args...) // exit 0
		assertEqual(t, label+": stderr", r.stderr, warnings(3, 4))
		if strings.Contains(r.stdout, "warning") || strings.Contains(r.stdout, "real date") {
			t.Errorf("%s: a warning reached stdout:\n%s", label, r.stdout)
		}
	}
	assertContains(t, "log show", e.mustRun("log", "show").stdout, "call the bank about 2026-13-45 ts:")

	// Commands that don't read the time log say nothing.
	for _, args := range [][]string{{"todo", "show"}, {"log", "edit", "--help"}} {
		assertEqual(t, "htt "+strings.Join(args, " ")+": stderr", e.mustRun(args...).stderr, "")
	}

	// log add reads the day's log before it appends; saving drops the
	// blank line, and later warnings name the lines as saved.
	r := e.mustRun("log", "add", "lunch")
	assertEqual(t, "log add: stderr", r.stderr, warnings(3, 4))
	r = e.mustRun("log", "show")
	assertEqual(t, "log show after log add: stderr", r.stderr,
		"warning: "+logFile+":2: 2026-13-45 isn't a real date, so it's text\n"+
			"warning: "+logFile+":3: due:friday isn't a YYYY-MM-DD date, so the task has no due date\n")
}

func TestRepl_TimeLogWarningsOncePerSession(t *testing.T) {
	e := newEnv(t)
	d := today()
	logFile := filepath.Join(e.dataDir, "timelogs", d+".log")
	e.writeData(filepath.Join("timelogs", d+".log"), "call the bank about 2026-13-45 ts:"+d+"T00:02:00Z\n")
	// The typed entry warns as it's logged; reading the log again in
	// the same session repeats neither warning.
	r := e.mustRunWithInput("/log status\n/log add read the 2026-5-3 minutes\n/log show\n/report\nq\n", "repl")
	assertEqual(t, "stderr", r.stderr,
		"warning: "+logFile+":1: 2026-13-45 isn't a real date, so it's text\n"+
			"warning: "+logFile+":2: 2026-5-3 isn't written YYYY-MM-DD (2026-05-03), so it's text\n")
}

func TestRepl_ATypedTaskWarnsOnce(t *testing.T) {
	// The REPL reloads the list after every command: the line warned
	// about as it was added doesn't warn again.
	e := newEnv(t)
	r := e.mustRunWithInput("add pay rent 2026-13-45\nshow\nq\n", "repl")
	assertEqual(t, "stderr", r.stderr,
		"warning: "+filepath.Join(e.dataDir, "todo.txt")+":1: 2026-13-45 isn't a real date, so it's text\n")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), today()+" pay rent 2026-13-45\n")
}
