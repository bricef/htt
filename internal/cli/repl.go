package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bricef/htt/internal/vars"
	"github.com/chzyer/readline"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// Repl runs an interactive todo-mode shell.
//
// Each line entered clears the screen, runs its command, and shows
// the command's output above the current context, which stays on
// screen until the next line. Inputs without a leading slash dispatch
// as `todo <input>` (todo-mode default); inputs starting with `/`
// have the slash stripped and dispatch as a full CLI command.
//
// Exit on Ctrl-D, Ctrl-C, or by typing `quit`, `exit`, or `q`. An
// empty Enter refreshes the view.
var Repl = &cobra.Command{
	Use:   "repl",
	Short: "Interactive todo-mode shell.",
	Long: `Interactive todo-mode shell.

Inputs are dispatched as ` + "`todo <input>`" + ` by default — typing
` + "`add buy milk`" + ` runs ` + "`htt todo add buy milk`" + `. Prefix a
line with ` + "`/`" + ` to escape to a full CLI command:
` + "`/log start review`" + ` runs ` + "`htt log start review`" + `.
` + "`/interactive`" + ` (or ` + "`/i`" + `) opens the TUI; quitting it comes
back to the REPL, in the context the TUI left current.

A command's output stays on screen, above the current context, until
the next command. Up/down arrows walk the commands typed, kept in
repl-history in the config directory (` + "`htt config dir`" + `).

Exit with Ctrl-D, Ctrl-C, or by typing quit / exit / q.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRepl()
	},
}

func init() {
	RootCmd.AddCommand(Repl)
	replRefused = []*cobra.Command{Repl, GUI}
}

// runRepl owns the readline loop. Errors from individual commands
// print to stderr but don't break the loop — only EOF, interrupt,
// or an exit word exits.
//
// The screen shows, top to bottom: the last command as typed and
// its output, the view, and the prompt. The view is either the
// current context (the default) or the REPL help text (after
// `/help`); any dispatch returns the user to the context view.
//
// Entering a line clears the screen before its command runs, not
// after, so the command's output stays on screen until the next
// line; the view is drawn after the command, so it shows the
// command's effect. An empty line just redraws the view.
//
// `/interactive` runs the TUI on the REPL's terminal, which the REPL
// hands over for the run (replTerminal.handOver).
func runRepl() error {
	rt := &replTerminal{open: openReplReader, isTerminal: replOnTerminal}
	lines, err := rt.open()
	if err != nil {
		return err
	}
	rt.lines = lines
	defer func() { _ = rt.lines.Close() }()
	prevRunOnTerminal := runOnTerminal
	runOnTerminal = rt.handOver
	defer func() { runOnTerminal = prevRunOnTerminal }()

	showHelp := false
	renderView := func() {
		if showHelp {
			printReplHelp(os.Stdout)
		} else {
			renderReplView()
		}
	}

	_, _ = fmt.Fprint(os.Stdout, ansiClearScreen)
	renderView()
	for {
		prompt := replPrompt()
		rt.lines.SetPrompt(prompt)
		line, err := rt.lines.Readline()
		if errors.Is(err, io.EOF) || errors.Is(err, readline.ErrInterrupt) {
			fmt.Println()
			return nil
		}
		if err != nil {
			return fmt.Errorf("readline: %w", err)
		}

		kind, parsed := classifyReplInput(line)
		if kind == replInputExit {
			return nil
		}
		_, _ = fmt.Fprint(os.Stdout, ansiClearScreen)
		switch kind {
		case replInputHelp:
			showHelp = true
		case replInputDispatch:
			showHelp = false
			fmt.Println(prompt + strings.TrimSpace(line))
			if name := refusedInRepl(parsed); name != "" {
				fmt.Fprintf(os.Stderr, "%q is not available inside the REPL\n", name)
			} else {
				resetReplFlags()
				if err := Execute(parsed); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			}
		}
		// replInputNone, an empty Enter, refreshes whichever view is
		// current.
		renderView()
	}
}

type replInputKind int

const (
	replInputNone replInputKind = iota
	replInputExit
	replInputHelp
	replInputDispatch
)

// classifyReplInput maps a raw input line to the action the loop
// should take. Pure function for testability.
//
//   - empty / whitespace-only      → (none, nil)
//   - "quit" / "exit" / "q"        → (exit, nil)
//   - "/help" (with any trailers)  → (help, nil)
//   - "/<rest>"                    → (dispatch, replWords(rest)), or (none, nil) if rest is empty
//   - "<anything>"                 → (dispatch, ["todo", ...replWords(line)])
func classifyReplInput(line string) (replInputKind, []string) {
	s := strings.TrimSpace(line)
	if s == "" {
		return replInputNone, nil
	}
	switch s {
	case "quit", "exit", "q":
		return replInputExit, nil
	}
	if strings.HasPrefix(s, "/") {
		rest := strings.TrimSpace(s[1:])
		fields := replWords(rest)
		if len(fields) == 0 {
			return replInputNone, nil
		}
		if fields[0] == "help" {
			return replInputHelp, nil
		}
		return replInputDispatch, fields
	}
	return replInputDispatch, append([]string{"todo"}, replWords(s)...)
}

// replRefused holds the commands that don't make sense inside the
// REPL: `repl` itself (a REPL inside the REPL) and `gui` (it serves
// until Ctrl-C, which would end the REPL too). init fills it: naming
// Repl here would be an initialization cycle, as Repl's RunE leads to
// refusedInRepl.
var replRefused []*cobra.Command

// refusedInRepl returns the name of the command args run when the REPL
// refuses it, or "". Cobra finds the command, as it would to run it, so
// an alias is refused with its command.
func refusedInRepl(args []string) string {
	cmd, _, err := RootCmd.Find(args)
	if err != nil || !slices.Contains(replRefused, cmd) {
		return ""
	}
	return cmd.Name()
}

// lineReader is what the REPL uses of readline: replReader, or a
// stand-in in tests.
type lineReader interface {
	Readline() (string, error)
	SetPrompt(prompt string)
	Close() error
}

// replTerminal is the REPL's hold on its terminal: the line reader
// reading it, and what opens another.
type replTerminal struct {
	lines      lineReader
	open       func() (lineReader, error)
	isTerminal func() bool // whether the REPL reads and writes a terminal
}

// handOver lends the terminal to run, the TUI, for as long as it runs.
// It closes the line reader first, which stops the goroutines readline
// reads the terminal on, so nothing but the TUI reads it while it
// runs. After run, even when it fails, a new line reader opens on the
// same history file. The REPL then draws its view and prompt, as after any command:
// the context the TUI left current, with its tasks as the TUI left
// them. When no new reader opens, the REPL ends at its next read, with
// the reason.
//
// A REPL reading a pipe has no terminal to lend: the TUI doesn't run,
// and gets an error saying why.
func (rt *replTerminal) handOver(run func() error) error {
	if !rt.isTerminal() {
		return errors.New("the TUI needs a terminal, and the REPL's input or output isn't one")
	}
	var closeErr error
	if err := rt.lines.Close(); err != nil {
		closeErr = fmt.Errorf("close readline: %w", err)
	}
	runErr := run()
	lines, err := rt.open()
	if err != nil {
		lines = failedReader{fmt.Errorf("reopen after the TUI: %w", err)}
	}
	rt.lines = lines
	return errors.Join(closeErr, runErr)
}

// failedReader stands in for a line reader that didn't open: reading
// from it fails, which ends the REPL.
type failedReader struct{ err error }

func (f failedReader) Readline() (string, error) { return "", f.err }
func (failedReader) SetPrompt(string)            {}
func (failedReader) Close() error                { return nil }

// replOnTerminal reports whether the REPL reads from and writes to a
// terminal, as the TUI needs.
func replOnTerminal() bool {
	return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd())
}

// openReplReader opens a readline instance on stdin, with its history
// in repl-history.
func openReplReader() (lineReader, error) {
	stdin := readline.Stdin
	config := &readline.Config{
		Prompt:          replPrompt(),
		HistoryFile:     replHistoryPath(),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		// replReader.Readline saves each line.
		DisableAutoSaveHistory: true,
	}
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		// Lines from a pipe or a file: read them as they are, with no
		// prompt or echo. readline does this by itself on Unix. On
		// Windows it reads key presses from the console and takes
		// stdin to be one, so a pipe ended the REPL before its first
		// line.
		stdin = os.Stdin
		config.FuncIsTerminal = func() bool { return false }
	}
	// readline wraps stdin in this reader itself when given none, and
	// doesn't close it.
	cancelable := readline.NewCancelableStdin(stdin)
	config.Stdin = cancelable
	rl, err := readline.NewEx(config)
	if err != nil {
		_ = cancelable.Close()
		return nil, fmt.Errorf("init readline: %w", err)
	}
	return replReader{Instance: rl, stdin: cancelable}, nil
}

// replReader is a readline instance as the REPL reads with it.
type replReader struct {
	*readline.Instance
	stdin io.Closer
}

// Readline reads a line and saves it to the history. readline saves a
// line itself on a goroutine of its own, after returning it, which
// could come after the reader is closed for the TUI, losing the
// `/interactive` that started it.
func (r replReader) Readline() (string, error) {
	line, err := r.Instance.Readline()
	if err == nil {
		_ = r.SaveHistory(line) // an error writing the file; readline ignores it too
	}
	return line, err
}

// Close closes readline, then the stdin reader it read through.
func (r replReader) Close() error {
	err := r.Instance.Close()
	_ = r.stdin.Close()
	return err
}

// resetReplFlags zeroes the package-level Cobra flag globals so
// state from a prior loop iteration can't bleed into the next
// dispatch. Cobra writes the flag's value into the bound variable
// during Parse but doesn't reset to the default when the flag is
// absent — so a single --due / --since from one command would
// stick across subsequent runs without this reset.
//
// New stateful flags added elsewhere need to be added here.
func resetReplFlags() {
	addDue = ""
	addToDue = ""
	logAt = atFlag{}
	reportSince, reportSinceGiven = "7d", false
	reportWeek, reportMonth = false, false
	showSortKey = ""
	showFilters = nil
	tuiLogFile = ""
	tuiLogLevel = "debug"
	guiPort = 0
	guiNoOpen = false
}

// ansiClearScreen homes the cursor and clears the entire visible
// screen. readline.ClearScreen only emits the cursor-home half of
// this sequence (\033[H) and leaves any pre-existing content on
// screen — so if the new render is shorter than the previous one,
// trailing characters from the prior task list bleed through.
const ansiClearScreen = "\033[H\033[2J"

// renderReplView prints the current context. Errors here print but
// don't break the loop — a load failure shouldn't lock the user out
// of the prompt.
func renderReplView() {
	ctx, err := repo().CurrentContext()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not load current context:", err)
		return
	}
	ctx.Show()
}

// printReplHelp writes the REPL keymap to w. Curated rather than
// generated from Cobra so the message can highlight REPL-specific
// behaviour (todo-mode default, `/` escape, exit words) that Cobra
// wouldn't know to mention. For per-command details the user runs
// `/<command> --help`.
func printReplHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, replHelpText)
}

const replHelpText = `htt REPL — todo-mode shell

Common commands (run implicitly as todo <command>):
  add <entry...>            add a task to the current context
  delete <index>            archive the task at <index>
  do <index>                complete the task
  up <index>                raise priority one step
  down <index>              lower priority one step
  priority <index> <A|B|C>  set priority explicitly
  move <index> <context>    move task to another context
  replace <index> <entry>   replace the task
  context [name]            show or switch context
  show                      re-render the view (or hit Enter on empty line)
  search <regex>            filter the current view
  random                    pick a task at random
  edit <index>              open the task in your editor
  edit-done                 open done.txt in your editor
  edit-archive              open archive.txt in your editor

REPL controls:
  /<command> [args...]      run any htt command (e.g. /log start, /report)
  /interactive, /i          open the TUI; quit it (q) to come back here
  /help                     this message
  quit, exit, q             leave the REPL (Ctrl-D / Ctrl-C also work)

/repl and /gui don't run inside the REPL.

Quotes group words, as in a shell: /log end --at "yesterday 17:30".
An apostrophe in a word (Sam's) is kept as typed.

Your editor is $VISUAL, or $EDITOR when $VISUAL isn't set or TERM is
dumb or unset, as in git; EDITOR="code --wait" works.

For per-command details, use /<command> --help (e.g. /todo add --help).`

// replPrompt returns the per-cycle prompt string. The context name
// is fetched live so it tracks `context <name>` switches.
func replPrompt() string {
	name, err := repo().CurrentContextName()
	if err != nil || name == "" {
		name = "?"
	}
	return fmt.Sprintf("htt(%s)> ", name)
}

// replHistoryPath returns a usable history file path in the config
// directory, or "" to disable history.
//
// Resolution order: the config directory (vars.ConfigDir, the
// directory holding config.yaml), then $HOME/.config/htt, then
// disabled.
func replHistoryPath() string {
	dir := vars.ConfigDir()
	if dir == "" {
		if h, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(h, vars.DefaultConfigDir)
		}
	}
	if dir == "" {
		return ""
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	return filepath.Join(dir, "repl-history")
}
