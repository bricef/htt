package editor

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// getenv looks variables up in vars, as os.Getenv does in the
// environment: a variable that isn't there reads as "".
func getenv(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

const (
	notSet     = "neither $VISUAL nor $EDITOR is set"
	notSetDumb = "$EDITOR is not set ($VISUAL isn't used when TERM is dumb)"
)

func TestChoose(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want Editor
		err  string // "" when Choose should find an editor
	}{
		{"EDITOR alone", map[string]string{"EDITOR": "vi"}, Editor{"EDITOR", "vi"}, ""},
		{"VISUAL alone", map[string]string{"VISUAL": "code --wait"}, Editor{"VISUAL", "code --wait"}, ""},
		{"VISUAL wins over EDITOR", map[string]string{"VISUAL": "code --wait", "EDITOR": "vi", "TERM": "xterm-256color"}, Editor{"VISUAL", "code --wait"}, ""},
		{"VISUAL wins with TERM unset", map[string]string{"VISUAL": "code", "EDITOR": "vi"}, Editor{"VISUAL", "code"}, ""},
		{"TERM=dumb skips VISUAL", map[string]string{"VISUAL": "code", "EDITOR": "vi", "TERM": "dumb"}, Editor{"EDITOR", "vi"}, ""},
		{"only TERM=dumb exactly is dumb", map[string]string{"VISUAL": "code", "EDITOR": "vi", "TERM": "dumb-emacs"}, Editor{"VISUAL", "code"}, ""},
		{"empty VISUAL falls back to EDITOR", map[string]string{"VISUAL": "", "EDITOR": "vi"}, Editor{"EDITOR", "vi"}, ""},
		{"blank VISUAL falls back to EDITOR", map[string]string{"VISUAL": " \t", "EDITOR": "vi"}, Editor{"EDITOR", "vi"}, ""},
		{"the value is kept as set", map[string]string{"EDITOR": " vim -u NONE "}, Editor{"EDITOR", " vim -u NONE "}, ""},
		{"neither set", map[string]string{}, Editor{}, notSet},
		{"both empty", map[string]string{"VISUAL": "", "EDITOR": ""}, Editor{}, notSet},
		{"both blank", map[string]string{"VISUAL": " ", "EDITOR": "\n"}, Editor{}, notSet},
		{"TERM=dumb and only VISUAL", map[string]string{"VISUAL": "code", "TERM": "dumb"}, Editor{}, notSetDumb},
		{"TERM=dumb and neither", map[string]string{"TERM": "dumb"}, Editor{}, notSetDumb},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Choose(getenv(tc.env))
			if tc.err != "" {
				if err == nil || err.Error() != tc.err {
					t.Fatalf("Choose() = %+v, %v; want error %q", got, err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Choose(): %v", err)
			}
			if got != tc.want {
				t.Errorf("Choose() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// shell is the argv git builds for a value it runs through the shell.
func shell(value, path string) []string {
	return []string{"/bin/sh", "-c", value + ` "$@"`, value, path}
}

func TestArgv_UnixExecsAPlainValueAndShellsTheRest(t *testing.T) {
	const path = "/home/me/my tasks/todo.txt"
	cases := []struct {
		value string
		want  []string
	}{
		{"vi", []string{"vi", path}},
		{"/usr/bin/vim", []string{"/usr/bin/vim", path}},
		{"code --wait", shell("code --wait", path)},
		{"subl -w", shell("subl -w", path)},
		{"emacsclient -t -a ''", shell("emacsclient -t -a ''", path)},
		{"vim -u NONE", shell("vim -u NONE", path)},
		{`"/opt/my editor/ed" --wait`, shell(`"/opt/my editor/ed" --wait`, path)},
		{"$HOME/bin/ed", shell("$HOME/bin/ed", path)},
		{" vi", shell(" vi", path)},
	}
	for _, goos := range []string{"linux", "darwin"} {
		for _, tc := range cases {
			got, err := Editor{"EDITOR", tc.value}.argv(goos, path)
			if err != nil {
				t.Errorf("%s: argv(%q): %v", goos, tc.value, err)
				continue
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("%s: argv(%q) = %q, want %q", goos, tc.value, got, tc.want)
			}
		}
	}
}

func TestArgv_UnixShellsExactlyGitsMetacharacters(t *testing.T) {
	// prepare_shell_cmd in git's run-command.c: a value with any of
	// these runs through the shell, any other value is executed.
	const path = "/tmp/todo.txt"
	for _, c := range "|&;<>()$`\\\"' \t\n*?[#~=%" {
		value := "ed" + string(c) + "x"
		got, _ := Editor{"EDITOR", value}.argv("linux", path)
		if want := shell(value, path); !slices.Equal(got, want) {
			t.Errorf("argv(%q) = %q, want the shell: %q", value, got, want)
		}
	}
	for _, c := range ":/-._+,@!{}]^é" {
		value := "ed" + string(c) + "x"
		got, _ := Editor{"EDITOR", value}.argv("linux", path)
		if want := []string{value, path}; !slices.Equal(got, want) {
			t.Errorf("argv(%q) = %q, want it executed: %q", value, got, want)
		}
	}
}

func TestArgv_WindowsSplitsWordsWithDoubleQuotes(t *testing.T) {
	const path = `C:\Users\me\my tasks\todo.txt`
	cases := []struct {
		value string
		want  []string // nil when argv should fail
		err   string
	}{
		{"notepad", []string{"notepad", path}, ""},
		{"code --wait", []string{"code", "--wait", path}, ""},
		{`"C:\Program Files\Microsoft VS Code\bin\code.cmd" --wait`,
			[]string{`C:\Program Files\Microsoft VS Code\bin\code.cmd`, "--wait", path}, ""},
		{`C:\tools\vim.exe -u NONE`, []string{`C:\tools\vim.exe`, "-u", "NONE", path}, ""},
		{`C:\"Program Files"\ed.exe`, []string{`C:\Program Files\ed.exe`, path}, ""},
		{`ed a"b c"d`, []string{"ed", "ab cd", path}, ""},
		{`emacsclient -a ""`, []string{"emacsclient", "-a", "", path}, ""},
		{"  code \t --wait  ", []string{"code", "--wait", path}, ""},
		{`ed 'two words'`, []string{"ed", "'two", "words'", path}, ""},
		{`ed $HOME`, []string{"ed", "$HOME", path}, ""},
		{`"C:\Program Files\code.cmd --wait`, nil, `$EDITOR has an unclosed quote: "C:\Program Files\code.cmd --wait`},
		{`code "--wait`, nil, `$EDITOR has an unclosed quote: code "--wait`},
	}
	for _, tc := range cases {
		got, err := Editor{"EDITOR", tc.value}.argv("windows", path)
		if tc.want == nil {
			if err == nil || err.Error() != tc.err {
				t.Errorf("argv(%q) = %q, %v; want error %q", tc.value, got, err, tc.err)
			}
			continue
		}
		if err != nil {
			t.Errorf("argv(%q): %v", tc.value, err)
			continue
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("argv(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

func TestArgv_ABlankValueIsAnError(t *testing.T) {
	// Choose never returns one; a hand-made Editor mustn't panic.
	for _, goos := range []string{"linux", "windows"} {
		if got, err := (Editor{"EDITOR", " "}).argv(goos, "/tmp/todo.txt"); err == nil {
			t.Errorf("%s: argv of a blank value = %q, want an error", goos, got)
		}
	}
}

// recorder writes a script into dir that records the arguments it's
// run with, one per line in brackets, to the file it returns.
func recorder(t *testing.T, dir string) (script, args string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script = filepath.Join(dir, "record.sh")
	args = filepath.Join(dir, "args")
	body := "#!/bin/sh\nprintf '[%s]\\n' \"$@\" > '" + args + "'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script, args
}

func TestCommand_RunsTheValueAsGitWould(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("runs a /bin/sh script")
	}
	plain, plainArgs := recorder(t, t.TempDir())
	spacedDir := filepath.Join(t.TempDir(), "my editors")
	spaced, spacedArgs := recorder(t, spacedDir)
	t.Setenv("HTT_TEST_EDITORS", spacedDir)
	file := filepath.Join(t.TempDir(), "my tasks.txt")

	cases := []struct {
		value string
		args  string // the file the editor records to
		want  string
	}{
		{plain, plainArgs, "[" + file + "]\n"},
		{plain + " --wait", plainArgs, "[--wait]\n[" + file + "]\n"},
		{plain + " -t -a ''", plainArgs, "[-t]\n[-a]\n[]\n[" + file + "]\n"},
		{plain + ` --title "two words"`, plainArgs, "[--title]\n[two words]\n[" + file + "]\n"},
		{"HTT_TEST_X=1 " + plain, plainArgs, "[" + file + "]\n"},
		{"'" + spaced + "' -w", spacedArgs, "[-w]\n[" + file + "]\n"},
		{`"$HTT_TEST_EDITORS/record.sh" -u NONE`, spacedArgs, "[-u]\n[NONE]\n[" + file + "]\n"},
	}
	for _, tc := range cases {
		_ = os.Remove(tc.args)
		cmd, err := Editor{"EDITOR", tc.value}.Command(file)
		if err != nil {
			t.Errorf("Command(%q): %v", tc.value, err)
			continue
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("EDITOR=%q: %v\n%s", tc.value, err, out)
			continue
		}
		got, err := os.ReadFile(tc.args)
		if err != nil {
			t.Errorf("EDITOR=%q: the editor recorded nothing: %v", tc.value, err)
			continue
		}
		if string(got) != tc.want {
			t.Errorf("EDITOR=%q: the editor got\n%s\nwant\n%s", tc.value, got, tc.want)
		}
	}
}

func TestCommand_KeepsAnEditorThatWontStartAnError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("runs /bin/sh")
	}
	missing := filepath.Join(t.TempDir(), "no-such-editor")
	for _, value := range []string{missing, missing + " --wait", "false", "false --wait"} {
		cmd, err := Editor{"EDITOR", value}.Command("/tmp/todo.txt")
		if err != nil {
			t.Errorf("Command(%q): %v", value, err)
			continue
		}
		if out, err := cmd.CombinedOutput(); err == nil {
			t.Errorf("EDITOR=%q ran cleanly, want an error\n%s", value, out)
		} else if strings.Contains(value, "--wait") && !strings.Contains(err.Error(), "exit status") {
			t.Errorf("EDITOR=%q: %v, want sh's exit status", value, err)
		}
	}
}
