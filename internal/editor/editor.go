// Package editor builds the command that opens a file in the user's
// editor, choosing the editor and running it as git does. It only
// builds the command: the CLI runs it on its terminal, and the TUI
// hands it to Bubble Tea, which gives the editor the terminal first.
package editor

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Editor is the editor the environment names: the variable it came
// from, without the $ ("VISUAL" or "EDITOR"), and its value as set.
type Editor struct {
	Var   string
	Value string
}

var (
	errNotSet     = errors.New("neither $VISUAL nor $EDITOR is set")
	errNotSetDumb = errors.New("$EDITOR is not set ($VISUAL isn't used when TERM is dumb or unset)")
)

// Choose returns the editor getenv's environment names, as git picks
// one: $VISUAL, unless TERM is dumb or (outside Windows) unset, then
// $EDITOR. A variable that's unset, empty or only whitespace doesn't
// count. getenv is os.Getenv outside tests. The only error is that no
// editor is set.
func Choose(getenv func(string) string) (Editor, error) {
	return choose(getenv, runtime.GOOS)
}

// choose is Choose for goos, which decides what an unset TERM means.
func choose(getenv func(string) string, goos string) (Editor, error) {
	term := getenv("TERM")
	// As git's is_terminal_dumb, an unset TERM is dumb, except on Windows,
	// where consoles leave it unset and Git for Windows sets one itself.
	dumb := term == "dumb" || (term == "" && goos != "windows")
	if v := getenv("VISUAL"); !dumb && strings.TrimSpace(v) != "" {
		return Editor{Var: "VISUAL", Value: v}, nil
	}
	if v := getenv("EDITOR"); strings.TrimSpace(v) != "" {
		return Editor{Var: "EDITOR", Value: v}, nil
	}
	if dumb {
		return Editor{}, errNotSetDumb
	}
	return Editor{}, errNotSet
}

// Command returns the command that opens path in e, ready to run with
// the caller's own stdin, stdout and stderr; see argv for how it's
// built. The only error is a Windows value with an unclosed quote.
func (e Editor) Command(path string) (*exec.Cmd, error) {
	args, err := e.argv(runtime.GOOS, path)
	if err != nil {
		return nil, err
	}
	return exec.Command(args[0], args[1:]...), nil
}

// shellChars are the characters that make git run an editor through
// the shell (prepare_shell_cmd in git's run-command.c).
const shellChars = "|&;<>()$`\\\"' \t\n*?[#~=%"

// argv returns the program and arguments that open path in e on goos.
//
// Everywhere but Windows, it's what git runs: a value without spaces
// or shell characters is the program, given path as its argument; any
// other value runs through /bin/sh as `sh -c '<value> "$@"' <value>
// <path>`, so arguments, quotes and variables in it work as in a shell
// and path arrives as one argument, whatever it holds.
//
// Windows has no sh, so the value is split into words there (see
// splitWindows) and the first runs with the rest, then path.
func (e Editor) argv(goos, path string) ([]string, error) {
	if strings.TrimSpace(e.Value) == "" {
		return nil, fmt.Errorf("$%s is empty", e.Var)
	}
	if goos == "windows" {
		words, err := splitWindows(e.Value)
		if err != nil {
			return nil, fmt.Errorf("$%s %w: %s", e.Var, err, e.Value)
		}
		return append(words, path), nil
	}
	// ":" is git's "don't run an editor": sh runs it, and it does nothing.
	if e.Value != ":" && !strings.ContainsAny(e.Value, shellChars) {
		return []string{e.Value, path}, nil
	}
	return []string{"/bin/sh", "-c", e.Value + ` "$@"`, e.Value, path}, nil
}

var errUnclosedQuote = errors.New("has an unclosed quote")

// splitWindows splits s into words at spaces and tabs. A double quote
// starts or ends a quoted part, whose spaces belong to the word, and
// is itself dropped, so `"C:\Program Files\code.cmd" --wait` is two
// words and `""` an empty one. Backslashes and single quotes are
// ordinary characters, as they are in Windows paths.
func splitWindows(s string) ([]string, error) {
	var words []string
	var word strings.Builder
	inWord, quoted := false, false
	for _, r := range s {
		switch {
		case r == '"':
			quoted = !quoted
			inWord = true
		case !quoted && (r == ' ' || r == '\t' || r == '\n' || r == '\r'):
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	if quoted {
		return nil, errUnclosedQuote
	}
	if inWord {
		words = append(words, word.String())
	}
	return words, nil
}
