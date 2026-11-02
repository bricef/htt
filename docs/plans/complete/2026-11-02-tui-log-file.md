# `htt interactive --log-file / --log-level`

**Date:** 2026-11-02
**Status:** Complete (2026-11-02)

## Outcome

Shipped as planned. Beyond the plan:

- **An architecture test** (`TestArchitecture_TUIDoesNotUseTheGlobalLogger`)
  fails if `internal/tui` imports `log` again, so a stray `log.Printf`
  can't quietly put stderr output back over the screen.
- **`AddTask`'s silent failures** (it never quit, it just dropped the
  task) are logged at error too, and adding a task in the done tab
  logs at warn.
- **Checked on a real terminal.** A pty driver ran the same keys
  (down, complete, done tab, complete) through the old and new
  binaries: the old one wrote 139 bytes of `action:` lines to stderr,
  the new one none. With `--log-file` the log held start, each action,
  the done-tab warning and exit, in a `0600` file; `--log-level warn`
  kept only the warning. The TUI test suites, which used to print 12
  `action:` lines per run, print none.

## Motivation

The TUI logs through Go's process-wide `log` package, which writes to
stderr unless redirected. Its one log call
(`log.Printf("action: %s", …)` on every keypress) therefore lands on
the terminal the TUI is drawing on, scribbling over the screen. The
only way to divert it is `--debug`, which points the whole process's
logger at `./debug.log` in whatever directory htt was started from.

Meanwhile the log misses what's most worth recording: about 15 action
failure paths end in a bare `return m, tea.Quit`, so when the TUI
quits unexpectedly nothing says why. The TUI test suites also leak a
dozen `action:` lines to stderr on every run.

## Decisions captured

From the analysis and design dialogue (2026-11-02):

1. **The TUI gets its own logger.** `tui.Model` takes a
   `*slog.Logger`; the TUI no longer touches the process-wide `log`
   package. The CLI builds the logger from flags. `utils.Info` /
   `Success` / `Fatal` (CLI status messages on the global logger) are
   untouched: they're user-facing output, not TUI diagnostics.

2. **Silent by default.** Without `--log-file` the logger discards
   everything (`slog.DiscardHandler`). Nothing reaches stderr.

3. **`--log-file PATH` appends** (create if missing, mode `0600`),
   using slog's text format so `tail -f` and `grep` work. If the file
   can't be opened, the command fails before the TUI starts.

4. **`--log-level debug|info|warn|error`, default `debug`.** The
   default is stated in the flag help and the docs. The flag only
   means something alongside `--log-file`; given alone it's an error
   rather than silently ignored. An unknown level is an error too,
   checked before the file is opened.

5. **`--debug` / `-d` is removed.** `--log-file debug.log` replaces
   it; the README's TUI dev loop is updated.

6. **Each level has defined content:**

   | Level   | Records                                                        |
   | ------- | -------------------------------------------------------------- |
   | `debug` | every action the TUI runs (each keypress)                      |
   | `info`  | the TUI starting (with its context) and exiting                |
   | `warn`  | an action that doesn't apply in the current view               |
   | `error` | an action that failed, with the error; most also close the TUI |

7. **Actions report failures; `Action.Act` logs them.** Action
   functions return an `error` alongside the model and command. `Act`
   is the one place that logs: every action at debug, an
   `errUnavailable`-wrapped error at warn, anything else at error.
   The action still chooses what the TUI does next, so quit/stay-open
   behaviour is unchanged; only the logging is new.

8. **Flags live on `interactive` only**, not on the root command.

## Shape

- `internal/tui/actions.go`: `mkAction` takes
  `func(model) (tea.Model, tea.Cmd, error)`; `Act` logs as above;
  `errUnavailable` sentinel; each failure path returns a wrapped error
  naming what it was doing.
- `internal/tui/interactive.go`: `model.logger`; `Model(repo, ctx,
  logger)` (nil means discard); the `log.Printf` in `Update` goes, as
  `Act` now logs every action, including the text-input ones (add
  task, cancel) that `Update` never logged.
- `internal/cli/interact.go`: `--log-file`, `--log-level`, a
  `tuiLogger(path, level)` helper, info lines around `p.Run()`, a
  `Long` description with the level table.
- `internal/cli/repl.go`: `resetReplFlags` resets the two new flag
  globals.

## Tests

- `internal/tui`: a buffer-backed logger proves debug / warn / error
  routing through `Act` (a keypress, completing a task in the done
  tab, completing a task in an empty context).
- `internal/cli`: `tuiLogger` with no file (discard), a bad level, a
  level filter, file mode, append across opens, an unopenable path.
- `test/e2e`: flag validation fails before the TUI starts (no TTY
  needed): `--log-level` alone, a bad level (and no file created), an
  unopenable `--log-file`.
- `test/tui` and `internal/tui` construct models with a nil logger,
  so the suites stop writing to stderr.

## Docs

README dev loop, the GETTING-STARTED interactive-mode section (flag
usage plus the level table), and the command's `--help`.

## Out of scope

- **`EditFile` and the terminal.** The edit action runs `$EDITOR`
  without the TUI releasing the terminal, and `utils.Fatal` on a
  missing `$EDITOR` exits inside the alt screen. The fix is
  `tea.ExecProcess`; separate change.
- **Separating CLI status messages from the global logger.**
- **Showing an action's error after the TUI quits**, or not quitting
  at all. Behaviour stays as it is; this change only records it.
- **A config-file key for the log file.** Flag only for now.
