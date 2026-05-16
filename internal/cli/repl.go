package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bricef/htt/internal/vars"
	"github.com/chzyer/readline"
	"github.com/spf13/cobra"
)

// Repl runs an interactive todo-mode shell.
//
// Each cycle clears the screen, renders the current context, and
// reads one command line. Inputs without a leading slash dispatch
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

Exit with Ctrl-D, Ctrl-C, or by typing quit / exit / q.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRepl()
	},
}

func init() {
	RootCmd.AddCommand(Repl)
}

// runRepl owns the readline loop. Errors from individual commands
// print to stderr but don't break the loop — only EOF, interrupt,
// or an exit word exits.
//
// The view shown above the prompt is either the current context
// (the default) or the REPL help text (after `/help`). Any
// dispatch returns the user to the context view on the next
// cycle.
func runRepl() error {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          replPrompt(),
		HistoryFile:     replHistoryPath(),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return fmt.Errorf("init readline: %w", err)
	}
	defer func() { _ = rl.Close() }()

	showHelp := false
	for {
		rl.SetPrompt(replPrompt())
		if showHelp {
			_, _ = fmt.Fprint(os.Stdout, ansiClearScreen)
			printReplHelp(os.Stdout)
		} else {
			renderReplView()
		}

		line, err := rl.Readline()
		if errors.Is(err, io.EOF) || errors.Is(err, readline.ErrInterrupt) {
			fmt.Println()
			return nil
		}
		if err != nil {
			return fmt.Errorf("readline: %w", err)
		}

		kind, parsed := classifyReplInput(line)
		switch kind {
		case replInputNone:
			// Empty Enter refreshes whichever view is current.
			continue
		case replInputExit:
			return nil
		case replInputHelp:
			showHelp = true
			continue
		case replInputDispatch:
			showHelp = false
			if isDisabledInRepl(parsed) {
				fmt.Fprintf(os.Stderr, "%q is not available inside the REPL\n", parsed[0])
				continue
			}
			resetReplFlags()
			RootCmd.SetArgs(parsed)
			if err := RootCmd.Execute(); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
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
//   - "/<rest>"                    → (dispatch, strings.Fields(rest))
//                                    or (none, nil) if rest is empty
//   - "<anything>"                 → (dispatch, ["todo", ...fields])
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
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return replInputNone, nil
		}
		if fields[0] == "help" {
			return replInputHelp, nil
		}
		return replInputDispatch, fields
	}
	return replInputDispatch, append([]string{"todo"}, strings.Fields(s)...)
}

// isDisabledInRepl rejects commands that don't make sense inside
// the REPL — `interactive` (would launch the TUI on top of the
// REPL's terminal) and `repl` itself (recursive REPL).
func isDisabledInRepl(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "interactive", "repl":
		return true
	}
	return false
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
	reportSince = "7d"
}

// ansiClearScreen homes the cursor and clears the entire visible
// screen. readline.ClearScreen only emits the cursor-home half of
// this sequence (\033[H) and leaves any pre-existing content on
// screen — so if the new render is shorter than the previous one,
// trailing characters from the prior task list bleed through.
const ansiClearScreen = "\033[H\033[2J"

// renderReplView clears the screen and prints the current context.
// Errors here print but don't break the loop — a load failure
// shouldn't lock the user out of the prompt.
func renderReplView() {
	_, _ = fmt.Fprint(os.Stdout, ansiClearScreen)
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
  + <index>                 raise priority
  - <index>                 lower priority
  priority <index> <A|B|C>  set priority explicitly
  move <index> <context>    move task to another context
  replace <index> <entry>   replace the task
  context [name]            show or switch context
  show                      re-render the view (or hit Enter on empty line)
  search <regex>            filter the current view
  random                    pick a task at random
  edit <index>              open the task in $EDITOR
  edit-done                 open done.txt in $EDITOR
  edit-archive              open archive.txt in $EDITOR

REPL controls:
  /<command> [args...]      run any htt command (e.g. /log start, /report)
  /help                     this message
  quit, exit, q             leave the REPL (Ctrl-D / Ctrl-C also work)

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

// replHistoryPath returns a usable history file path under the
// configured config directory, or "" to disable history.
//
// Resolution order: viper's config_path key (matches the rest of
// the config-aware code), then $HOME/.config/htt, then disabled.
func replHistoryPath() string {
	dir := vars.Get(vars.ConfigKeyConfigPath)
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
