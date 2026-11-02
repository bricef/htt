# TUI: hand the terminal to `$EDITOR` properly

**Date:** 2026-11-02
**Status:** In progress
**Builds on:** [2026-11-02-tui-log-file.md](../complete/2026-11-02-tui-log-file.md)

## Motivation

The TUI's edit-file action (`f`) calls `utils.EditFilePath`, which
runs `$EDITOR` directly while Bubble Tea still owns the terminal: the
screen stays in alt-screen raw mode and Bubble Tea's input reader
keeps reading the keyboard, competing with the editor for keys.
Worse, its failure paths call `utils.Fatal`, which `os.Exit`s from
inside the TUI: an unset `$EDITOR`, an editor that won't start, or one
that exits non-zero kills htt and leaves the terminal in alt-screen
mode with the message lost.

## Decisions captured

From the design dialogue (2026-11-02):

1. **Run the editor through `tea.ExecProcess`.** Bubble Tea leaves the
   alt screen, stops reading input, runs the editor on the real
   terminal, restores the screen, then sends a message carrying the
   editor's error, if any.

2. **The editor never closes the TUI.** Each failure is reported on a
   status line and logged; the TUI carries on:

   | Situation                        | Status line             | Log   |
   | -------------------------------- | ----------------------- | ----- |
   | `$EDITOR` unset or empty         | asks you to set it      | warn  |
   | context has no file (memory repo)| says there's no file    | warn  |
   | editor fails to start            | the start error         | error |
   | editor exits non-zero            | the exit status         | error |

   The no-file case used to quit; it now follows the same rule.

3. **Reload after every editor session, failed or not.** A failing
   editor may still have written the file, and the view must match
   disk. A reload that itself fails keeps today's behaviour (log at
   error, quit): staying open on a stale context would let the next
   keypress save over the user's edits.

4. **A one-line status line** above the footer, rendered only when
   there's a message, cleared on the next keypress.

5. **`$EDITOR` is still run as a single program name**, as the CLI's
   `htt todo edit` does. `EDITOR="code --wait"` therefore fails to
   start, now gracefully. Shell-style parsing (as git does) is a
   separate change.

## Shape

- `internal/tui/actions.go`: `EditFile` builds the command and returns
  `tea.ExecProcess(...)`; `editorFinishedMsg{editor, err}`;
  `editorFinished` reloads and reports. The tui package stops
  importing `utils`.
- `internal/tui/interactive.go`: `model.status`, cleared on each
  `tea.KeyMsg`; `Update` routes `editorFinishedMsg`; `View` renders the
  status line.
- Docs: GETTING-STARTED interactive section mentions `f` and what
  happens when the editor can't run; the warn-level wording in the
  docs and `htt interactive --help` widens to cover `$EDITOR`.

## Tests

- `internal/tui`: `EditFile` with `$EDITOR` empty, with no file, and
  with an editor set (returns an exec command, not quit);
  `editorFinishedMsg` for success (context reloaded from disk), a
  non-zero exit, and a start failure; status cleared by the next key;
  `View` shows the status.
- Real terminal (pty driver): unset `$EDITOR`, a missing editor, an
  editor exiting 1, and an editor that reads a line from the terminal
  and appends it to the file, which shows the terminal really is
  handed over. Same keys through the old binary for comparison.

## Out of scope

- Shell-style `$EDITOR` parsing and `$VISUAL`.
- `utils.EditFilePath` and the CLI edit commands (`todo edit-done`,
  `edit-archive`, `log edit`), which still `Fatal` on failure; fine for
  a one-shot CLI, and a separate cleanup.
- Explaining a quit after the TUI exits (printing the error that
  closed it). Separate change.
