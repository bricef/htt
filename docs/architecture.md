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
  repo/                    thin wrapper over go-git for `htt sync`
  timelogs/                (deleted; lives in domain + storage now)
  parseutils/              parsing helpers for the todo.txt format
  utils/                   small odds and ends (StringToFilename, EditFilePath, …)
  vars/                    viper config keys + defaults
test/
  e2e/                     black-box tests that drive the compiled binary
  tui/                     bubble tea snapshot tests
  architecture/            asserts the import-direction invariant
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

- `domain` imports nothing from the rest of `internal/`.
- `storage` may import `domain` only.
- `tui` may import `domain` and `storage`; never `cli`.
- `cli` sits at the top and may import anything below (including `tui`,
  to register the interactive subcommand).

`test/architecture/architecture_test.go` enforces this by scanning
imports in each package. If a future change violates the direction the
test fails and forces the discussion before the dependency lands.

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
is `Append`; pure methods are `Latest`, `Duration`, `IsEmpty`.

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

## CLI / TUI wiring

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

The TUI's `model` carries a `repo domain.Repository`. Actions like
`Delete`, `Do`, `IncreasePriority` call `m.context.X(...)` directly;
context-switching actions call `m.repo.SetCurrent`.

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
   (`withMemoryRepo` / `withMemoryTimelogRepo`). For TUI features, the
   snapshot suite under `test/tui/`.

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
