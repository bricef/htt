# Extract `TimelogRepository`, idiomize `internal/timelogs`

**Date:** 2026-05-15
**Status:** Proposed
**Builds on:** `2026-05-14-collapse-usecase-into-domain.md`

## Motivation

`internal/timelogs` is the last package still on the pre-refactor patterns
the rest of the codebase has shed:

- `utils.DieOnError` / `utils.Fatal` on every I/O error — the rest of the
  codebase moved to idiomatic `(value, error)` returns in `498c217`.
- Direct `os.OpenFile` / `os.ReadFile` calls + direct `vars.Get` reads.
  The Context refactor proved out a repository pattern; timelogs still
  reaches past it.
- A package-level function surface (`AddEntry`, `CurrentActive`,
  `CurrentDuration`, …) instead of a domain type with methods. There's
  nothing to inject and nothing to test in isolation without viper.

The collapse-usecase plan explicitly flagged this as the next refactor:
"`internal/timelogs/` keeps wrapping `domain.Task.Annotate` with
`utils.DieOnError`. A future refactor moves timelogs into the domain too."

## Decisions captured

These came out of the planning conversation; recording them here so they
don't have to be relitigated during execution.

1. **Split repository.** `TimelogRepository` is a separate interface from
   `domain.Repository`. Justification: timelogs may live under a different
   filesystem root from contexts (the current code already places them
   under `vars.ConfigKeyDataDir + vars.DefaultTimelogDirName`, and a
   future config split is plausible). Two repos is slightly more verbose
   at the CLI seam (two factory helpers, two `Set*` for tests) but
   cleanly separates the two persistence concerns.

2. **Operations live on a `Timelog` type, not the repo.** Mirrors the
   `Context` pattern: `tl.Append(task)`, `tl.Latest()`, `tl.Duration()`
   read like the user's mental model. The repo is the factory.

3. **Preserve naive "last entry is active" semantics, including
   `@end`.** Today `htt log end` writes an `@end` entry and the next
   `htt log status` reports "Currently working on @end (3m)". That's
   the documented (or at least observable) behaviour and a richer model
   (an `Open`/`Closed` flag per entry) is a feature, not a refactor.
   `Latest()` returns the last parsed Task as-is.

4. **Empty/missing log behaviour.** `Latest()` returns `nil` on an
   empty or missing log (preserves `bug_004` fix). `Duration()` returns
   `0` when `Latest()` is `nil`.

5. **`TimestampLabel` moves into `domain`.** The "ts" annotation key is
   a domain concept (Timelog.Append sets it; CLI's "log active"
   uses `RemoveAnnotation(label)` to print without the timestamp). The
   exported symbol becomes `domain.TimelogTimestampLabel`.

6. **`CurrentLogPath` stays.** The `htt log edit` command shells out to
   `$EDITOR` on today's log file. It needs an on-disk path. The path
   lives on the file-backed repo (parallel to `Context.Filepath`); a
   small accessor on `TimelogRepository` exposes it. Documented as
   "intended for the $EDITOR shellout only; programmatic edits go
   through `Timelog.Append`."

7. **File format is byte-exact.** One entry per line, `<task with
   ts:<RFC3339>>\n`. Matches the legacy on-disk format so existing
   timelog files migrate without conversion.

8. **`internal/timelogs/` disappears.** Its public surface is replaced
   by `domain.Timelog` + `domain.TimelogRepository`. The `cli/log.go`,
   `cli/begin.go`, `cli/status.go` callers update to the new API.

## Shape

```go
// internal/domain/timelog.go
//
// Timelog is one day's worth of activity entries. Each entry is a Task
// with a "ts:<RFC3339>" annotation set by Append.
type Timelog struct {
    Date    time.Time
    Entries []*Task
    repo    TimelogRepository
}

// Constructor for repo implementations only; external callers obtain
// Timelogs through TimelogRepository.Today / Day.
func NewTimelog(repo TimelogRepository, date time.Time) *Timelog

// Pure methods (no repo needed)
func (l *Timelog) Latest() *Task            // last entry or nil
func (l *Timelog) Duration() time.Duration  // since Latest's ts:, or 0
func (l *Timelog) IsEmpty() bool

// Persistent method
func (l *Timelog) Append(task *Task) (*Task, error)
```

```go
// internal/domain/timelog_repository.go
type TimelogRepository interface {
    // Today returns the current day's Timelog. Empty (no entries) for
    // a missing or empty file.
    Today() (*Timelog, error)

    // Day returns the Timelog for an arbitrary date. Same empty
    // behaviour for missing files.
    Day(date time.Time) (*Timelog, error)

    // Save persists a Timelog, overwriting any prior state for the
    // same date. Internal use by Timelog.Append; external callers
    // mutate through Timelog methods.
    Save(l *Timelog) error

    // CurrentLogPath returns the on-disk path for today's log file.
    // Used by `htt log edit` to hand a path to $EDITOR; not for
    // programmatic reads/writes.
    CurrentLogPath() string
}

const TimelogTimestampLabel = "ts"
```

```go
// internal/storage/timelog_memory.go
type MemoryTimelogRepository struct { ... }
func NewMemoryTimelogRepository() *MemoryTimelogRepository
// implements domain.TimelogRepository
```

```go
// internal/storage/timelog_file.go
type FileTimelogRepository struct {
    dataDir string  // root for the timelog subdirectory
}
func NewFileTimelogRepository(dataDir string) *FileTimelogRepository
// implements domain.TimelogRepository
```

```go
// internal/cli/app.go (additions)
var defaultTimelogRepo domain.TimelogRepository
func timelogRepo() domain.TimelogRepository { ... }
func SetTimelogRepository(r domain.TimelogRepository)
```

## Steps

Each step keeps `just test` green.

### Step 1 — Add `domain.Timelog` + `domain.TimelogRepository` interface

**Test harness:**
- `internal/domain/timelog_test.go` (white-box, `package domain`):
  pure-method tests against struct-literal `Timelog`s. Pins
  `Latest()` for empty / single-entry / multi-entry. Pins `Duration()`
  with a fixed-time fake. `TestNewTimelog_InjectsRepo` mirrors the
  Context wiring test.

**Refactor:**
- Create `internal/domain/timelog.go` with the `Timelog` struct,
  constructor, pure methods.
- Create `internal/domain/timelog_repository.go` with the interface +
  the `TimelogTimestampLabel` constant.
- `Timelog.Append` exists but is exercised in Step 2 (needs an impl).

**Verify:** domain tests green; nothing else changes.

**Risk:** none — purely additive.

---

### Step 2 — `MemoryTimelogRepository` + contract suite

**Test harness:**
- `internal/storage/timelog_contract_test.go` with `runTimelogRepositoryContract` exercising:
  - `Today` on empty store returns an empty Timelog (not nil, no error).
  - `Save` then `Today` round-trips entries.
  - `Save` preserves insertion order.
  - `Day(date)` returns the right slice for that date.
  - `Save` does not alias the caller's `Entries` slice.
- `TestMemoryTimelogRepository_Contract` runs the suite.

**Refactor:**
- Create `internal/storage/timelog_memory.go`.
- `MemoryTimelogRepository` keys an internal map by `date.Format("2006-01-02")`.

**Verify:** contract suite green for memory impl.

**Risk:** none — additive; no production callers yet.

---

### Step 3 — `FileTimelogRepository` against the same contract

**Test harness:**
- `internal/storage/timelog_file_test.go`:
  - `TestFileTimelogRepository_Contract` runs the contract suite.
  - `TestFileTimelogRepository_Save_ByteExactOutput` pins the on-disk
    format (one entry per line, `\n`-terminated, `ts:<RFC3339>` set by
    `Timelog.Append`).
  - `TestFileTimelogRepository_Today_OnMissingFile` returns an empty
    Timelog without an error (matches today's `utils.ReadLines`
    short-circuit semantics in `CurrentActive`).
  - `TestFileTimelogRepository_CurrentLogPath_UsesViperLayout` pins
    `<dataDir>/<DefaultTimelogDirName>/<YYYY-MM-DD>.log`.

**Refactor:**
- Create `internal/storage/timelog_file.go`. Mirrors the existing
  `FileRepository` shape (path builder, Read/Write helpers, `.bak`
  rotation is NOT required for timelogs — entries are append-mostly,
  not whole-file rewrites).
- Wait — `Save` IS whole-file: it serialises `tl.Entries` and writes.
  We DO want `.bak` rotation? Probably not, because timelog files are
  append-only in practice; the legacy code used `os.O_APPEND`. Decide
  during implementation: either match legacy append behaviour
  (`Save` re-opens with `O_APPEND` and writes just the new tail —
  needs diff tracking) or do whole-file rewrite-with-bak. The simpler
  shape is whole-file rewrite + bak. Document the trade-off in the
  commit.

**Verify:** file impl passes the same contract + the file-specific tests.

**Risk:** the file-format pin catches any format drift; the `.bak`
decision is reversible.

---

### Step 4 — Wire `timelogRepo()` into the CLI

**Test harness:**
- `internal/cli/todo_test.go` already has the in-process Cobra harness.
  Add a `withMemoryTimelogRepo(t)` helper alongside `withMemoryRepo(t)`
  for tests that drive the `log` subcommands.

**Refactor:**
- `internal/cli/app.go`: add `defaultTimelogRepo`, `timelogRepo()`,
  `SetTimelogRepository`. The default constructor uses
  `storage.NewFileTimelogRepository(vars.Get(vars.ConfigKeyDataDir))`.

**Verify:** existing CLI tests untouched and green.

**Risk:** none — additive.

---

### Step 5 — Port the `log` subcommands to the new API

**Test harness:**
- Existing `TestCobra_LogActive_HandlesEmptyLog` already covers the
  empty-log path. Extend it: add a counterpart that seeds an entry
  via the memory timelog repo and verifies `log active` prints the
  expected lines (using a captured-stdout harness or a behavioural
  proxy — the existing tests use the absence of an error as a smoke
  signal; we can do the same here).

**Refactor (per command):**

- `htt log add <raw>`:
  ```go
  tl, err := timelogRepo().Today()
  if err != nil { return fmt.Errorf("load today's log: %w", err) }
  task, err := domain.NewTask(raw)
  if err != nil { return fmt.Errorf("parse entry: %w", err) }
  if _, err := tl.Append(task); err != nil { return fmt.Errorf("append: %w", err) }
  fmt.Printf("Logging entry: %v\n", task.RemoveAnnotation(domain.TimelogTimestampLabel).ColorString())
  ```

- `htt log start` / `htt log end`: same as `add`, with `domain.NewTask("@start")` / `domain.NewTask("@end")`.

- `htt log show`:
  ```go
  tl, err := timelogRepo().Today()
  ...
  for _, e := range tl.Entries {
      fmt.Println(e.Raw)
  }
  ```

- `htt log edit`:
  ```go
  utils.EditFilePath(timelogRepo().CurrentLogPath())
  return nil
  ```

- `htt log status`:
  ```go
  tl, err := timelogRepo().Today()
  if err != nil { ... }
  latest := tl.Latest()
  if latest == nil {
      fmt.Println("Not currently working on any task.")
      return nil
  }
  fmt.Printf("Currently working on: %v (%v) \n",
      latest.RemoveAnnotation(domain.TimelogTimestampLabel).ColorString(),
      utils.HumanizeDuration(tl.Duration()))
  ```

- `htt log active`: identical to `status` except the existing wording.

- `htt status` (the outer `htt status` that also calls `timelogs.ShowStatus`): updates to load the timelog and print the same lines inline.

- `htt workon <id>` (`internal/cli/begin.go`): currently calls
  `timelogs.AddEntry(t)` after finding the task. Updates to load today's
  timelog and Append.

**Verify:**
- e2e CLI tests untouched (they black-box the binary's output).
- In-process Cobra tests pass.
- Manual smoke (or new behavioural test) of `htt log start`, `htt log status`, `htt log end`, `htt log status` to confirm the @end-is-active behaviour is preserved.

**Risk:** the eight call sites (across `log.go`, `begin.go`, `status.go`)
are a wide rename surface; mitigated by doing them all in one commit
behind a green build.

---

### Step 6 — Delete `internal/timelogs/`

**Refactor:**
- `rm internal/timelogs/timelogs.go`.
- Remove the `internal/timelogs` import from `cli/log.go`, `cli/begin.go`,
  `cli/status.go` (already replaced in Step 5).
- Architecture test: no row to remove (timelogs wasn't constrained
  before). Verify nothing imports the deleted package.

**Verify:** all suites green; `go vet ./...` clean.

**Risk:** none — purely additive deletion after Step 5.

---

## Out of scope

- Richer entry model (explicit Open/Closed flag, pause semantic). The
  refactor preserves observable behaviour; a real pause feature is its
  own piece of work.
- Sealing `Timelog.Entries`. The Context refactor decided
  `Tasks` stays exported for read-only callers (rendering); Timelog
  follows the same convention.
- Splitting the timelog data dir from the context data dir. The two
  repos accept distinct paths today; defaulting them to the same root
  via `vars.ConfigKeyDataDir` matches current behaviour. A future
  `ConfigKeyTimelogDir` is a one-line follow-up.
- `htt log edit` going through anything richer than a $EDITOR shellout.
  The `CurrentLogPath` accessor is the minimum that keeps that flow
  working; a structured editing UI is a feature.
