package e2e

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

// htt picks the editor as git does: $VISUAL unless TERM is dumb, then
// $EDITOR. On Linux and macOS a value with spaces or shell characters
// (EDITOR="code --wait") runs through sh, with the file as its last
// argument; before, htt ran the whole value as a program name, so
// such values failed to start and $VISUAL was ignored.

// fakeEditor writes an editor script into dir that records the
// arguments it gets, one per line in brackets, to the file it
// returns, and writes "edited by <label>" into the file it's given
// (its last argument). It writes only to an absolute path, so a broken
// launcher that hands it the wrong last argument (--wait) can't leave
// files in the test's directory.
func fakeEditor(t *testing.T, dir, label string) (script, args string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake editor is a /bin/sh script")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script = filepath.Join(dir, "editor.sh")
	args = filepath.Join(dir, "args")
	body := "#!/bin/sh\n" +
		"printf '[%s]\\n' \"$@\" > '" + args + "'\n" +
		"for last; do :; done\n" +
		"case $last in /*) echo 'edited by " + label + "' > \"$last\" ;; esac\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script, args
}

// readArgs returns what the fake editor recorded, and removes it so
// the next run starts clean.
func readArgs(t *testing.T, args string) string {
	t.Helper()
	b, err := os.ReadFile(args)
	if err != nil {
		t.Fatalf("the editor recorded no arguments: %v", err)
	}
	_ = os.Remove(args)
	return string(b)
}

func TestEditor_AValueWithArgumentsRuns(t *testing.T) {
	e := newEnv(t)
	script, args := fakeEditor(t, e.home, "code")
	tmp := t.TempDir()
	e.extraEnv = []string{"EDITOR=" + script + " --wait", "TMPDIR=" + tmp}
	e.mustRun("todo", "add", "a task")

	r := e.mustRun("todo", "edit", "0")
	assertContains(t, "todo edit: stdout", r.stdout, "After:  edited by code")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), "edited by code\n")
	got := readArgs(t, args)
	if !regexp.MustCompile(`^\[--wait\]\n\[` + regexp.QuoteMeta(tmp) + `/hypothetical-tracker-todo\d+\]\n$`).MatchString(got) {
		t.Errorf("todo edit: the editor got %q, want --wait and the task's temp file", got)
	}

	e.mustRun("todo", "edit-done")
	assertEqual(t, "edit-done: arguments", readArgs(t, args), "[--wait]\n["+filepath.Join(e.dataDir, "done.txt")+"]\n")
	assertEqual(t, "done.txt", e.readData("done.txt"), "edited by code\n")

	e.mustRun("log", "start")
	e.mustRun("log", "edit")
	logFile := filepath.Join("timelogs", today()+".log")
	assertEqual(t, "log edit: arguments", readArgs(t, args), "[--wait]\n["+filepath.Join(e.dataDir, logFile)+"]\n")
	assertEqual(t, "log file", e.readData(logFile), "edited by code\n")
}

func TestEditor_APathWithASpaceArrivesAsOneArgument(t *testing.T) {
	e := newEnv(t)
	e.dataDir = filepath.Join(e.home, "my tasks")
	e.writeConfig("data_path: " + e.dataDir + "\n")
	script, args := fakeEditor(t, e.home, "plain")
	spacedScript, spacedArgs := fakeEditor(t, filepath.Join(e.home, "my editors"), "spaced")
	e.mustRun("todo", "add", "a task")
	done := filepath.Join(e.dataDir, "done.txt")

	cases := []struct {
		editor string
		args   string
		want   string
	}{
		{script, args, "[" + done + "]\n"},                                  // executed directly
		{script + " --wait", args, "[--wait]\n[" + done + "]\n"},            // through sh
		{"'" + spacedScript + "' -w", spacedArgs, "[-w]\n[" + done + "]\n"}, // a quoted program with a space
	}
	for _, tc := range cases {
		e.extraEnv = []string{"EDITOR=" + tc.editor}
		e.mustRun("todo", "edit-done")
		assertEqual(t, "EDITOR="+tc.editor+": arguments", readArgs(t, tc.args), tc.want)
	}
}

func TestEditor_VisualWinsUnlessTheTerminalIsDumb(t *testing.T) {
	e := newEnv(t)
	script, args := fakeEditor(t, e.home, "either")
	e.mustRun("todo", "add", "a task")
	done := filepath.Join(e.dataDir, "done.txt")

	cases := []struct {
		name string
		env  []string
		want string // the editor's first argument
	}{
		{"VISUAL wins", []string{"TERM=xterm", "VISUAL=" + script + " visual", "EDITOR=" + script + " editor"}, "visual"},
		{"VISUAL alone", []string{"TERM=xterm", "VISUAL=" + script + " visual"}, "visual"},
		{"TERM=dumb uses EDITOR", []string{"TERM=dumb", "VISUAL=" + script + " visual", "EDITOR=" + script + " editor"}, "editor"},
		{"an empty VISUAL", []string{"TERM=xterm", "VISUAL=", "EDITOR=" + script + " editor"}, "editor"},
	}
	for _, tc := range cases {
		e.extraEnv = tc.env
		e.mustRun("todo", "edit-done")
		assertEqual(t, tc.name+": arguments", readArgs(t, args), "["+tc.want+"]\n["+done+"]\n")
	}
}
