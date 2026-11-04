# Architecture

This is the longer-form orientation for `htt`. The README covers what
the binary does and how to install it; this doc covers how the code is
laid out, the contracts between layers, and the path a new feature
takes through the system.

## Layout

```
cmd/htt/                   main package: builds the binary, wires viper config
internal/
  cli/                     cobra commands; one *.go per subcommand group
  tui/                     bubble tea interactive mode
  domain/                  Context, Task, Timelog + Repository / TimelogRepository interfaces
  storage/                 MemoryRepository / FileRepository (and timelog impls) — satisfy the domain interfaces
  repo/                    thin wrapper over go-git for `htt sync` (push-only; see #12)
  update/                  the once-a-day check for a newer GitHub release
  parseutils/              parsing helpers for the todo.txt format
  utils/                   small odds and ends (StringToFilename, EditFilePath, Fatal, …)
  vars/                    viper config keys + defaults
  gcal.go                  standalone Google Calendar experiment (package main);
                           not part of htt, nothing imports it
test/
  e2e/                     black-box tests that drive the compiled binary
  tui/                     TUI tests that drive the model through Update/View
  architecture/            asserts the import rules below
docs/
  architecture.md          you are here
  plans/active/            in-flight design docs
  plans/complete/          historical record of merged design docs
  archive/                 older notes preserved for context
```

## Layering and the architecture test

The dependency graph runs one way:

```
cli ─────────────► tui
 │                  │
 ▼                  ▼
storage ─────────► domain
```

`parseutils`, `vars` and `utils` are leaf helpers: they import nothing
else from `internal/`, and the layers below may use them, with the
exceptions noted.

- `domain` imports only `parseutils` and `vars`; never `storage`,
  `cli`, `tui` or `utils`.
- `storage` imports `domain` (plus `utils` and `vars`); never `cli` or
  `tui`.
- `tui` imports `domain` (it may also use `storage`); never `cli`,
  never `utils` (whose editor and `Fatal` helpers exit the process and
  write to the terminal the TUI owns), and never the standard `log`
  package (see [TUI](#tui)).
- `cli` sits at the top and may import anything below (including `tui`,
  to register the interactive subcommand).
- `update` imports nothing else from `internal/`: it knows GitHub's
  release endpoint and its cache file, nothing about tasks or config.
  `cli` decides when it may run and passes the TUI a notice source.
- `repo` (sync) sits outside the layering: it wraps go-git, uses
  `utils` and `vars`, and only `cli` calls it.

`test/architecture/architecture_test.go` enforces the `domain`,
`storage`, `tui` and `update` rules by scanning imports in each
package. If a
future change breaks one, the test fails and forces the discussion
before the dependency lands.

## Domain types

### `Context`

A named bundle of `Task`s. The repo is injected at construction (the
`repo` field is private) and is the seam through which persistent
methods save changes back. Pure methods (`Search`, `Sort`,
`GetTaskById`, …) never touch the repo, so tests for those can
construct `Context`s via struct literals.

Persistent methods that go through the repo:

- `AddTask(*Task) error`
- `Delete(strID string) (*Task, error)`
- `Replace(strID string, replacement *Task) (snapshot *Task, error)`
- `Move(strID, toName string) (*Task, error)`
- `Complete(strID string) (*Task, error)`
- `SetPriority(strID, priority string) (snapshot, mutated *Task, error)`
- `IncreasePriority(strID string) (snapshot, mutated *Task, error)`
- `DecreasePriority(strID string) (snapshot, mutated *Task, error)`

`Move` and `Complete` save destination-first so a partial-save failure
leaves the task in both places (recoverable) rather than neither
(silent data loss).

### `Task`

A single todo.txt entry. Constructors and presentation methods
(`NewTask`, `Entry`, `RawString`, `ConsoleString`, `Annotate`,
`RemoveAnnotation`) are exported. The mutator helpers (`setPriority`,
`increasePriority`, `decreasePriority`, `markCompleted`) are
unexported — the `Context` orchestrators are the only sanctioned
mutation path, since reaching for `Task` directly skips the persist
step.

### `Timelog`

A day's worth of activity entries. Each entry is a `Task` annotated
with a `ts:<RFC3339>` timestamp by `Timelog.Append`. Persistent method
is `Append`; pure methods are `Latest`, `Duration`, `IsEmpty`, and
`Spans` (the closed intervals between consecutive entries, which
`htt report` sums).

"Latest" returns whatever was last appended, including `@end` — so
`htt log status` after `htt log end` reports "Currently working on
@end (Xm)". That's the current product behaviour; sentinel-aware
semantics (an explicit Open/Closed flag per entry) is feature work.

## Repositories

Two interfaces in `domain`:

```go
type Repository interface {
    Context(name string) (*Context, error)
    Contexts() ([]*Context, error)
    ContextNames() ([]string, error)
    CurrentContext() (*Context, error)
    CurrentContextName() (string, error)
    SetCurrent(name string) error
    Save(ctx *Context) error
    ContextPath(name string) string
}

type TimelogRepository interface {
    Today() (*Timelog, error)
    Day(date time.Time) (*Timelog, error)
    Save(l *Timelog) error
    CurrentLogPath() string
}
```

Two implementations of each in `storage`:

- `MemoryRepository` / `MemoryTimelogRepository` — in-memory, for
  tests. They deep-copy entries on Save and Day so mutations through a
  returned pointer can't leak back into stored state. This matches the
  file impl's serialize-then-reparse semantics.
- `FileRepository` / `FileTimelogRepository` — todo.txt-format files
  on disk. The on-disk layout is byte-for-byte compatible with the
  pre-refactor format. Context names are sanitized
  (`utils.StringToFilename`) at the path boundary so a name like
  `../escape` can't write outside `dataDir`. Save rotates the existing
  file to `.bak` for one slot of crash recovery.

`FileRepository` takes two directories: `dataDir` for per-context
files and `pointerDir` for the `current-context` pointer. Default
config maps both to the same directory, but users who set
`tracker_path` independently of `data_path` keep their split.

## CLI wiring

`internal/cli/app.go` is the injection seam:

```go
func repo() domain.Repository
func SetRepository(r domain.Repository)

func timelogRepo() domain.TimelogRepository
func SetTimelogRepository(r domain.TimelogRepository)
```

Each is lazily initialized from viper config. Tests inject memory
impls via `SetRepository` / `SetTimelogRepository`. Cobra commands
call `repo()` / `timelogRepo()` at the top of each `RunE`.

## TUI

`internal/tui` is a Bubble Tea program. `tui.Model(repo, ctx, opts)`
builds the model from `Options` (a logger and a theme; the zero value
means a discard logger and the default theme). `htt interactive`
(`internal/cli/interact.go`) builds both, the logger from
`--log-file` / `--log-level` and the theme from the config, then runs
the program.

- **Actions.** Each key binding maps to an `Action`: a description
  plus a `func(model) (tea.Model, tea.Cmd, error)`. The action decides
  what happens next (carry on, `tea.Quit`, `tea.ClearScreen`, run a
  program); `Action.Act` is the one place that logs it: every action
  at debug, an error wrapping `errUnavailable` (the action can't run
  right now) at warn, any other error at error. Mutating actions call
  `m.context.X(...)`, which persists through the repo, then `refresh`;
  context-switching actions call `m.repo.SetCurrent`.
- **The terminal.** The TUI owns the terminal while it runs, so
  nothing in `internal/tui` writes to stdout or stderr: diagnostics go
  to the model's `*slog.Logger`, and anything the user should see goes
  on the status line (`model.status`, cleared on the next keypress).
  The architecture test keeps `log` and `utils` out of the package.
- **Themes.** `Theme` holds one colour per role; the built-in palettes
  live in `theme.go`. `cli` reads the config's `tui.theme` and
  `tui.colors` and calls `tui.ResolveTheme`, so the tui package never
  reads config, and passes the result in `Options`. The model builds
  its `styles` from it once; `View` takes every colour from there.
- **Other programs.** The edit action runs `$EDITOR` through
  `tea.ExecProcess`, which releases the terminal while the editor runs
  and sends `editorFinishedMsg` back to `Update` when it exits.

## Update check

`internal/update` asks GitHub for the latest release
(`releasesURL`, fixed at build time) and compares its tag with the
binary's version from Go's build info. `Start` records the attempt in
the user cache directory before the request and runs the request in
the background; `Check.Newer` reports a newer release only on the run
that asked, and every failure is kept in `Check.Err` rather than
returned. The root command's `PersistentPreRun` starts at most one
check per process when `updateCheckAllowed` permits (config on,
someone at a terminal, not completion or help); `main.go` prints the
notice after `Execute`, and `htt interactive` hands the TUI
`Options.UpdateNotice` for its status line. What it may contact and
send is promised in the README's Online activity section, and the
tests pin it.

## Sync

`htt sync` calls `repo.Sync()` in `internal/repo`, a thin layer over
go-git: init the data directory as a repository if needed, set the
remote, commit, push over SSH. It sits outside the domain/storage
layering, ends the process through `utils.Fatal` on any error, and has
no tests. What it does and doesn't do is in GETTING-STARTED's sync
section; its problems and the open questions about its future are in
[#12](https://github.com/bricef/htt/issues/12).

## Adding a feature

Most features touch one or two layers. The usual path:

1. **Domain change.** If the feature needs a new operation, add it as
   a method on `Context` / `Timelog` (or as a new domain type). Pure
   methods are testable with struct literals; persistent methods are
   testable through `MemoryRepository`.
2. **Repository contract.** If the feature needs new persistence (e.g.
   listing a new kind of entity), extend the interface and both impls.
   The contract test (`storage/contract_test.go`,
   `storage/timelog_contract_test.go`) is the place to pin behaviour
   both impls must satisfy.
3. **CLI / TUI surface.** Add a cobra subcommand or TUI action that
   wires the new domain method through `repo()` / `timelogRepo()`.
   Return errors via `RunE`; `main.go` owns error formatting.
4. **e2e or in-process Cobra test.** For CLI features, the lightest
   harness is `internal/cli/todo_test.go`'s in-process cobra tests
   (`withMemoryRepo` / `withMemoryTimelogRepo`); output belongs in the
   e2e suite, since in-process tests can't see stdout. For TUI
   features, `internal/tui/actions_test.go` (actions and their
   logging, against a memory or file repo) and `test/tui/` (drives the
   model through `Update` / `View`).

Always run `just test` before each commit. `just check` runs
golangci-lint for style/vet warnings.

## Why these choices

The detailed rationale for each piece lives in `docs/plans/complete/`.
Worth reading if you're touching the relevant area:

- `2026-05-14-business-layer-extraction.md` — why there's a
  Repository pattern at all, and the test harness that made the
  refactor safe.
- `2026-05-14-collapse-usecase-into-domain.md` — why the operations
  live on `Context` rather than in a separate service layer.
- `2026-05-15-timelogs-refactor.md` — why `TimelogRepository` is a
  separate interface from `Repository`, and why "last entry wins"
  semantics survived the refactor.
- `2026-11-02-tui-log-file.md` — why the TUI has its own logger and
  how actions report errors.
- `2026-11-02-tui-editor-exec.md` — how the editor gets the terminal,
  and why editor failures never close the TUI.
