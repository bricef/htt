# Collapse `usecase` into `domain`: operations as first-class Context methods

**Date:** 2026-05-14
**Status:** Proposed
**Builds on:** `2026-05-14-business-layer-extraction.md`

## Motivation

The previous refactor introduced `internal/usecase/` as the business layer that
orchestrates `domain` + `storage`. That move was correct as a transition, but
it leaves us with operations that are *conceptually* domain operations ("add a
task to a context", "complete a task") sitting outside the domain package.

The end state we want: those operations live as methods on the domain types
themselves. `ctx.AddTask(task)` reads like the user's mental model. The
`Repository` interface becomes a domain abstraction (DDD-correct). Storage just
provides implementations of it. The `usecase` package goes away.

## Shape

```go
// Repository is a domain abstraction; storage just implements it.
// internal/domain/repository.go
type Repository interface {
    LoadContext(name string) (*Context, error)
    SaveContext(*Context) error
    ListContextNames() ([]string, error)
    GetCurrentContextName() (string, error)
    SetCurrentContextName(name string) error
}

// Context carries an injected repo (via NewContext or LoadContext) and
// exposes the operations that need persistence.
type Context struct {
    Name  string
    Tasks []*Task
    repo  Repository // private, injected at construction
}

// Pure methods (work in memory; no repo needed)
func (c *Context) GetTaskById(int) (*Task, error)
func (c *Context) GetTaskByStrId(string) (*Task, error)
func (c *Context) GetTaskIndex(*Task) (int, error)
func (c *Context) Search(func(*Task) bool) []*Task
func (c *Context) Sort() *Context
func (c *Context) Equals(*Context) bool

// Persistent methods (require repo)
//
// Task construction is separate from placement — callers do
//     task, err := domain.NewTask("foo")
//     err = ctx.AddTask(task)
// rather than smushing both into one signature.
func (c *Context) AddTask(*Task) error
func (c *Context) Delete(strID string) (*Task, error)
func (c *Context) Replace(strID string, replacement *Task) (old *Task, err error)
func (c *Context) Move(strID, toName string) (*Task, error)
func (c *Context) Complete(strID string) (*Task, error)

// Persistent, mutation-only — no Task to construct, just transform existing.
// Internally calls the unexported task methods (see below) then saves.
func (c *Context) SetPriority(strID, p string) (old, new *Task, err error)
func (c *Context) IncreasePriority(strID string) (old, new *Task, err error)
func (c *Context) DecreasePriority(strID string) (old, new *Task, err error)
```

```go
// Task — pure value type. Mutation methods needed only by Context
// orchestrators become unexported (same-package tests can still exercise
// them). Constructors and presentation methods stay public.
type Task struct { ... }

func NewTask(raw string) (*Task, error)

// Public surface
func (t *Task) Entry() string
func (t *Task) RawString() string
func (t *Task) ConsoleString() string
func (t *Task) Annotate(key, value string) (*Task, error)     // timelogs uses this
func (t *Task) RemoveAnnotation(key string) *Task              // timelogs uses this

// Unexported — only Context orchestrators call these
func (t *Task) setPriority(p string) *Task
func (t *Task) increasePriority() *Task
func (t *Task) decreasePriority() *Task
func (t *Task) markCompleted(c *Context, when time.Time) (*Task, error)  // renamed from Do
```

## Why this is the right shape (recap of the design dialogue)

- **Why not "Tracker" service?** The operations either belong to a specific
  Context (add/delete/move/complete) or to the registry of contexts (list/
  current/switch). The Repository interface already represents the registry;
  Contexts already represent themselves. A separate Tracker would be a
  passthrough.

- **Why Context.AddTask(*Task) instead of Context.AddTask(rawString)?**
  Construction and placement are different concerns. Parse errors are caught at
  construction; placement errors are about the target context. Each method
  does one thing.

- **Why aren't priority mutations on Task directly?** State propagation. A
  `task.SetPriority(...)` that persists would need either a back-reference to
  its parent Context (cyclic graph, fragile after Move, forces tests to
  construct a full graph) or an explicit `ctx.Save()` call afterward (easy to
  forget; reintroduces the legacy "mutate-without-saving" footgun in a new
  form). Keeping Task pure and putting the orchestration on Context avoids
  both.

- **Why are `setPriority` / `increasePriority` / `decreasePriority` / `markCompleted` unexported?** Once persistence orchestration moves onto
  Context, external callers should never reach for the pure mutators on a
  Task they obtained from a Context — that path skips persistence. Making
  them unexported removes the footgun at the API surface. Same-package tests
  still exercise the pure semantics.

## Context/Repository inversion: the dependency mechanics

This refactor inverts a dependency: previously `Repository` (in storage)
referenced `Context`; now `Context` (in domain) holds a `Repository`. Both
the interface and the type live in `domain`. The mechanics:

**Intra-package cycle is fine.** `Repository` references `*Context` in its
method signatures, and `Context` has a `repo Repository` field. Go allows
mutual reference within a single package without import cycles.

**No cross-package cycle.** `storage` imports `domain` to access the
interface and the Context type. `domain` imports nothing from `storage`. The
concrete `*FileRepository` and `*MemoryRepository` types implement
`domain.Repository` by virtue of having the right method set — Go's
structural typing means the implementations don't need to know they're
implementing the interface.

**Storage can't touch Context's private `repo` field.** It's unexported, and
storage is in a different package. The injection point must be a
`domain`-defined constructor:

```go
// internal/domain/context.go
func NewContext(repo Repository, name string) *Context {
    return &Context{repo: repo, Name: name, Tasks: []*Task{}}
}

// internal/storage/file.go
func (r *FileRepository) LoadContext(name string) (*domain.Context, error) {
    ctx := domain.NewContext(r, name)
    // ... parse file, populate ctx.Tasks (exported field)
    return ctx, nil
}
```

`ctx.Tasks` stays public so storage can populate it directly after
construction. Only `repo` is private. This keeps the seam minimal.

**Pure tests construct via struct literal.** Tests that exercise only
pure methods (`GetTaskById`, `Search`, `Sort`, `Equals`) keep using
`&domain.Context{Name: ..., Tasks: ...}` — `repo` defaults to nil and the
pure methods never touch it. Tests that exercise persistent methods
construct via a memory repo: `repo := storage.NewMemoryRepository(); ctx, _ := repo.LoadContext("foo"); ctx.AddTask(...)`.

**Persistent methods assume non-nil repo.** Calling `ctx.AddTask(...)` on a
struct-literal Context with `repo == nil` will panic with a clear nil-
pointer dereference. This is a programmer error (you obtained a Context
without going through a repo), and we want it loud, not silent.

**The two construction paths converge.** The repo's `LoadContext` is the
ONLY way external callers get a Context that's wired for persistence.
`domain.NewContext` is the constructor storage uses internally; callers
outside the domain package shouldn't call it directly. Documented as
such.

## Steps

Each step keeps `just test` green.

### Step 1 — Move `Repository` interface to `internal/domain/`

**Test harness change:** none — interface relocation is purely mechanical.

**Refactor change:**
- Move `internal/storage/repository.go` → `internal/domain/repository.go`.
- Storage implementations import `domain.Repository`.
- Rename `Repository.ListContexts` → `ListContextNames` for consistency with
  the existing `GetCurrentContextName` / `SetCurrentContextName`. (The current
  method already returns `[]string`; the rename just makes the contract honest.)

**Verify:** all tests pass; architecture test confirms `storage` imports
`domain` (no new cycle).

**Risk:** none beyond a few import-path edits.

---

### Step 2 — Inject `Repository` into `Context`

See the "Context/Repository inversion" section above for the dependency
mechanics this step implements.

**Test harness change:**
- Pure-method tests in `internal/domain/context_test.go` continue to use
  `&Context{Name: ..., Tasks: ...}` struct literals (no repo).
- `TestContext_Add_DoesNotTouchDisk` and `TestContext_Remove_DoesNotTouchDisk`
  test the pure-helper semantics; they don't need a repo for the assertion
  (the assertion is "no file written") and can keep using struct literals.

**Refactor change:**
- Add private `repo Repository` field to `Context`.
- Change `NewContext(name string)` → `NewContext(repo Repository, name string)`.
  This is the seam through which storage injects the repo into a Context it's
  returning from `LoadContext`. Document NewContext as "intended for
  Repository implementations; external callers should obtain Contexts via
  `repo.LoadContext`".
- `FileRepository.LoadContext` and `MemoryRepository.LoadContext` change from
  ```go
  return &domain.Context{Name: name, Tasks: tasks}, nil
  ```
  to
  ```go
  ctx := domain.NewContext(r, name)
  ctx.Tasks = tasks
  return ctx, nil
  ```
  (`Tasks` is exported, so direct assignment is fine; only `repo` requires
  the constructor.)
- The two remaining external callers of `domain.NewContext` are the legacy
  `$EDITOR`-shelling commands that need a path string:
  - `internal/cli/todo.go`'s `edit-done` command: `domain.NewContext("done").Filepath()`
  - `internal/tui/actions.go`'s `EditFile` action: uses `m.context.Filepath()` (already gets a real Context from LoadContext — fine).
  The CLI's edit-done line becomes `domain.NewContext(repo(), "done").Filepath()`, or better, `ctx, _ := repo().LoadContext("done"); utils.EditFilePath(ctx.Filepath())`.

**Verify:** all tests green.

**Risk:** nil-repo panics on a struct-literal Context calling persistent
methods. Acceptable failure mode (programmer error, loud).

---

### Step 3 — Port use-case operations to Context methods

**Test harness change:** the existing 22 tests in `internal/usecase/usecase_test.go` migrate to `internal/domain/context_ops_test.go` and target
`Context` methods directly. They drive a memory repo + a loaded Context.

**Refactor change:** add the orchestrating methods on `Context`:

```go
func (c *Context) AddTask(*Task) error
func (c *Context) Delete(strID string) (*Task, error)
func (c *Context) Replace(strID string, new *Task) (old *Task, err error)
func (c *Context) Move(strID, toName string) (*Task, error)
func (c *Context) Complete(strID string) (*Task, error)
func (c *Context) SetPriority(strID, p string) (old, new *Task, err error)
func (c *Context) IncreasePriority(strID string) (old, new *Task, err error)
func (c *Context) DecreasePriority(strID string) (old, new *Task, err error)
```

Each method loads any auxiliary context via `c.repo.LoadContext`, applies the
mutation, and saves through `c.repo.SaveContext`. The snapshot pattern from
`swapTask` moves into the priority methods.

`usecase` package still exists in parallel during this step; CLI and TUI still
call `uc()`. Both layers coexist briefly.

**Verify:** new domain tests pass; existing usecase tests still pass (their
implementation is unchanged); e2e + TUI green.

**Risk:** divergence between the two implementations during the brief
coexistence. Mitigated by porting use case bodies verbatim.

---

### Step 4 — Rewire CLI and TUI to call `ctx.X(...)` directly

**Test harness change:** none — the existing `internal/cli/todo_test.go`,
`test/e2e/*`, `internal/tui/actions_test.go`, `test/tui/*` cover behavior.
Update them only if call shapes change.

**Refactor change:**
- Replace `cli.uc()` with `cli.repo()` (returning a `domain.Repository`).
- CLI commands change from `uc().AddTask(...)` to:
  ```go
  ctx, _ := repo().LoadContext(name)
  task, _ := domain.NewTask(raw)
  ctx.AddTask(task)
  ```
  ...or wrap that pattern in small CLI helpers if it shows up too often.
- TUI model holds a `domain.Repository` instead of `*usecase.UseCases`;
  actions call ctx methods.

**Verify:** e2e + TUI behavioral tests still green. In-process Cobra tests
update to use `SetRepository` instead of `SetUseCases`.

**Risk:** subtle call-site mistakes during the rewire. Mitigated by the e2e
harness as the safety net.

---

### Step 5 — Unexport Task mutators, rename `Do` → `markCompleted`

**Test harness change:** `internal/domain/task_test.go` lives in package
`domain`, so lowercase methods are reachable. Just rename the call sites in
the tests.

**Refactor change:**
- `SetPriority` → `setPriority`
- `IncreasePriority` → `increasePriority`
- `DecreasePriority` → `decreasePriority`
- `Do` → `markCompleted`

External callers (the ported Context methods from Step 3) update to lowercase.

**Verify:** `go build ./...` confirms no external caller reaches for these;
all tests green.

**Risk:** none beyond find-and-replace.

---

### Step 6 — Delete `internal/usecase/`, update architecture test

**Test harness change:** none new.

**Refactor change:**
- Delete `internal/usecase/usecase.go` and `usecase_test.go`. Their tests are
  already replicated in `internal/domain/context_ops_test.go` from Step 3.
- Architecture test drops the `internal/usecase` row.

**Verify:** all suites green; architecture test passes.

**Risk:** none — purely additive deletion once Steps 3 and 4 have done their
work.

---

## Out of scope

- `internal/timelogs/` keeps wrapping `domain.Task.Annotate` with
  `utils.DieOnError`. A future refactor moves timelogs into the domain too.
- `internal/repo/` (the git sync wrapper) untouched.
- Splitting `domain/` into `domain/task` and `domain/context` subpackages —
  the value isn't worth the import-path churn at this codebase size.
