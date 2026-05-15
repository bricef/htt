# Collapse `usecase` into `domain`: operations as first-class Context methods

**Date:** 2026-05-14
**Status:** Proposed
**Builds on:** `2026-05-14-business-layer-extraction.md`

## Resume marker (where this session left off)

- Branch: `refactor/collapse-usecase-into-domain`
- Step 1 complete (uncommitted): Repository moved to `internal/domain/`
  with the reshaped interface (`Context`, `Contexts`, `ContextNames`,
  `CurrentContext`, `CurrentContextName`, `SetCurrent`, `Save`). Storage
  impls renamed + extended. Sanitization moved into `SetCurrent`.
  `usecase.SwitchContext` is now a passthrough. Three new contract tests:
  `Contexts returns every persisted context with tasks loaded`,
  `CurrentContext defaults to todo and is loaded`,
  `SetCurrent sanitizes non-word characters`. All suites green
  (e2e, TUI, architecture, in-process Cobra, domain, storage, usecase).
- Next action: Step 2 — inject `Repository` into `Context`.

The plan went through two rounds of design dialogue before any code was
written. The "Decisions captured" section below records the choices that
came out of that dialogue so they don't have to be relitigated when work
resumes.

## Decisions captured during planning

These are the design choices that emerged from the conversation. Each is
reflected in the shape and steps below, but listed here in one place for
quick reference.

1. **The repo is the factory; Context is downstream.** Inversion direction:
   `Repository.Context(name)` returns a `*Context` wired with its repo.
   External callers never construct Contexts directly.

2. **`ctx.AddTask(task)` takes a pre-built Task, not a raw string.**
   Construction and placement are different concerns. Callers do
   `task, err := domain.NewTask("foo"); err = ctx.AddTask(task)`. Same
   pattern for `ctx.Replace(strID, newTask)`.

3. **Task mutation methods are unexported.** `SetPriority`,
   `IncreasePriority`, `DecreasePriority`, `Do` (renamed `markCompleted`)
   become lowercase. The Context layer is the only sanctioned mutation
   path; pure-mutation methods on Task exist for same-package tests and
   internal use only.

4. **Priority operations live on Context, not Task.** A `task.SetPriority`
   that persists would need a back-reference to its parent Context
   (cyclic graph, fragile after Move). Keeping mutation pure on Task and
   orchestration on Context avoids the trap.

5. **`Save` stays exported with a doc comment.** Go's unexported-method
   interface matching is single-package; an unexported `save` would
   prevent `storage.MemoryRepository` from satisfying `domain.Repository`.
   Doc comment communicates intent ("internal use by Context methods").
   No architecture-test enforcement — convention is enough.

6. **Two methods for listing contexts: `Contexts()` and `ContextNames()`.**
   Honest about cost. `ContextNames()` is cheap (just names, for tab
   strips and status). `Contexts()` loads all tasks (real use case:
   cumulative stats across all contexts).

7. **Two methods for the active context: `CurrentContext()` and `CurrentContextName()`.** Same shape. `CurrentContext()` defaults to the
   `"todo"` context if no pointer has been set — keeps setup costs to
   zero for new users.

8. **`SetCurrent(name)` sanitizes the input internally.** Non-word
   characters become underscores (preserving today's behavior via
   `utils.StringToFilename`). Returns just `error`. Callers that need the
   sanitized form follow up with `CurrentContextName()`. Sanitization
   moves from `usecase.SwitchContext` into the repo impls; the use case
   becomes a passthrough until it's deleted in Step 6.

9. **`Tasks []*Task` stays exported.** Repository impls need to populate
   it after construction. Fully sealing the type (private `tasks` with
   accessors) is out of scope here — possible follow-up.

10. **Storage impls live in `internal/storage/`, not collapsed into
    `domain`.** Keeps the layering from the previous refactor intact.
    The cost is `Save` being exported (per decision 5); the benefit is a
    smaller, more focused `domain` package.

11. **`internal/timelogs` stays out of scope.** Continues wrapping domain
    errors with `utils.DieOnError` for legacy parity. Its own refactor
    is a future improvement.

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

The Repository is the top of the object graph. A repo *holds* contexts.
That matches both the on-disk layout (a data dir contains context files)
and the user's mental model. Construction goes through the repo as factory:
external callers never write `domain.NewContext(...)` — they ask the repo
for a Context.

```go
// internal/domain/repository.go
//
// Repository is a domain abstraction; storage implementations satisfy it.
type Repository interface {
    // Context returns the named context, loaded with its tasks. A name
    // that has never been persisted returns an empty Context (Tasks empty).
    Context(name string) (*Context, error)

    // Contexts returns every persisted context with tasks loaded. Heavier
    // than ContextNames; use only when you actually need the task lists.
    Contexts() ([]*Context, error)

    // ContextNames returns the names of every persisted context. Cheap.
    // Use this for tab strips, status output, anywhere you just need to
    // list what's available.
    ContextNames() ([]string, error)

    // CurrentContext returns the active context, loaded. Equivalent to
    // Context(CurrentContextName()) but expressed as one call.
    CurrentContext() (*Context, error)

    // CurrentContextName returns just the name of the active context.
    // Defaults to DefaultContextName if no current pointer is set.
    CurrentContextName() (string, error)

    // SetCurrent persists the active-context pointer. Sanitization of the
    // name (non-word characters → underscores) lives here.
    SetCurrent(name string) error

    // Save persists a context. Intended for internal use by Context's
    // mutation methods (AddTask, Delete, etc.); external callers should
    // mutate through those methods rather than calling Save directly.
    // Public because Go's unexported-method interface matching only works
    // within a single package, and we want storage impls to live in
    // internal/storage/.
    Save(c *Context) error
}

// Context carries an injected repo (set by repo.Context() and friends) and
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
the interface and the type live in `domain`.

**Intra-package cycle is fine.** `Repository` references `*Context` in its
method signatures, and `Context` has a `repo Repository` field. Go allows
mutual reference within a single package without import cycles.

**No cross-package cycle.** `storage` imports `domain` to access the
interface and the Context type. `domain` imports nothing from `storage`. The
concrete `*FileRepository` and `*MemoryRepository` types in `internal/
storage/` implement `domain.Repository` by virtue of having the right
method set — Go's structural typing means the implementations don't need
to know they're implementing the interface.

**`Save` is exported, by convention private.** Go's unexported-method
interface matching only works within a single package, so an unexported
`save` method on `domain.Repository` would prevent `storage.MemoryRepository`
from satisfying it. We expose `Save` and document it as "intended for
internal use by Context methods; external callers should mutate through
the Context API." The architecture test could enforce this convention
(no caller outside `internal/domain/` should reference `Repository.Save`)
if it becomes load-bearing.

**Repo as factory; storage uses a domain-internal constructor.** Storage
can't set Context's private `repo` field directly, so the construction
seam is a domain-package constructor that storage calls:

```go
// internal/domain/context.go — used by repository implementations only
func NewContext(repo Repository, name string) *Context {
    return &Context{repo: repo, Name: name, Tasks: []*Task{}}
}

// internal/storage/file.go
func (r *FileRepository) Context(name string) (*domain.Context, error) {
    ctx := domain.NewContext(r, name)
    // ... parse file, populate ctx.Tasks (exported field)
    return ctx, nil
}
```

External callers never write `domain.NewContext(...)` — they ask the repo:
`repo.Context("todo")`. The constructor is documented as
"intended for Repository implementations; external callers obtain Contexts
via Repository.Context() or Repository.Contexts()."

**`Tasks` stays exported.** Repository impls populate it directly after
construction. Only `repo` is private. The implicit invariant is "after
NewContext, you set Tasks before returning the Context to a caller."

**Pure tests construct via struct literal.** Tests that exercise only
pure methods (`GetTaskById`, `Search`, `Sort`, `Equals`) keep using
`&domain.Context{Name: ..., Tasks: ...}` — `repo` defaults to nil and the
pure methods never touch it. Tests that exercise persistent methods
construct via a memory repo: `repo := storage.NewMemoryRepository(); ctx, _ := repo.Context("foo"); ctx.AddTask(...)`.

**Persistent methods assume non-nil repo.** Calling `ctx.AddTask(...)` on a
struct-literal Context with `repo == nil` will panic with a nil-pointer
dereference. This is a programmer error (you obtained a Context without
going through a repo), and we want it loud, not silent.

**Cross-context ops read naturally.** `c.Move("0", "work")` internally
calls `c.repo.Context("work")` to obtain the target Context (which is
wired with the same repo). No ambiguity about which repo to save to.

**`Tasks` could become unexported later.** With `AddTask` as the only
sanctioned mutation path, the exported `Tasks` field is no longer
load-bearing for the use case layer (which used to splice it directly to
avoid the legacy `Add`-triggers-`Sync` quirk). We keep `Tasks` exported
in this refactor for compatibility with repo impls and read-only callers
(rendering); fully sealing it is a future improvement out of scope here.

## Steps

Each step keeps `just test` green.

### Step 1 — Move `Repository` to `internal/domain/` with the inverted shape

This is a bigger step than I initially scoped: not just relocating the
interface, but reshaping it. We do the reshape and the move together so
storage impls only change once.

**Test harness change:**
- `internal/storage/contract_test.go` keeps the same shape ("Save then load
  round-trips", "Save preserves order", etc.) but updated method names:
  `LoadContext` → `Context`, `SaveContext` → `Save`, `ListContexts` →
  `ContextNames`, `GetCurrentContextName` → `CurrentContextName`,
  `SetCurrentContextName` → `SetCurrent`. Two new contract tests for the
  added methods: `Contexts()` returns all persisted contexts with tasks;
  `CurrentContext()` returns the active context (default `todo`) loaded.

**Refactor change:**
- Delete `internal/storage/repository.go`. Create `internal/domain/repository.go` with the inverted interface:
  ```go
  type Repository interface {
      Context(name string) (*Context, error)
      Contexts() ([]*Context, error)
      ContextNames() ([]string, error)
      CurrentContext() (*Context, error)
      CurrentContextName() (string, error)
      SetCurrent(name string) error
      Save(c *Context) error
  }
  ```
  Also moves `ErrInvalidContextName` and `DefaultContextName` constants.
- `internal/storage/memory.go` rewires:
  - `LoadContext(name)` → `Context(name)`
  - `SaveContext(c)` → `Save(c)`
  - `ListContexts()` → `ContextNames()`
  - `GetCurrentContextName()` → `CurrentContextName()`
  - `SetCurrentContextName(name)` → `SetCurrent(name)`
  - New: `Contexts() ([]*Context, error)` — iterate names, call Context for each
  - New: `CurrentContext() (*Context, error)` — call CurrentContextName, then Context
- `internal/storage/file.go` mirrors the same renames + additions.
- `internal/usecase/usecase.go` updates its repo calls accordingly. This is
  the one consumer that needs to swallow the rename; the use case package
  still exists during this step and remains the layer CLI/TUI call into.
- CLI and TUI don't touch `Repository` directly yet (still go through `uc()`).

**Sanitization moves to `SetCurrent`.** Currently
`usecase.SwitchContext` calls `utils.StringToFilename(rawName)` before
delegating to `repo.SetCurrentContextName`. After this step,
`Repository.SetCurrent` does the sanitization itself (so any caller — not
just the use case — gets the right behavior). The use case's
`SwitchContext` becomes a one-line passthrough; Step 4 deletes the
use-case wrapper.

**Verify:**
- All contract tests pass against both Memory and File impls.
- All existing e2e, TUI, in-process Cobra, domain, and usecase tests pass.
- Architecture test still passes (storage → domain remains the only
  cross-package direction).

**Risk:** the rename surface is wide (5 methods across 2 impls + 1 consumer
+ contract test). Mitigated by doing them all in one commit so the build is
green at every checkpoint.

---

### Step 2 — Inject `Repository` into `Context`

See "Context/Repository inversion" above for the dependency mechanics.

**Test harness change:**
- Pure-method tests in `internal/domain/context_test.go` continue using
  `&Context{Name: ..., Tasks: ...}` struct literals (no repo).
- `TestContext_Add_DoesNotTouchDisk` and `TestContext_Remove_DoesNotTouchDisk`
  use struct literals.

**Refactor change:**
- Add private `repo Repository` field to `Context`.
- Change `NewContext(name string)` → `NewContext(repo Repository, name string)`.
  Documented as "intended for Repository implementations; external callers
  obtain Contexts via `Repository.Context()` or `Repository.Contexts()`."
- `FileRepository.Context()` and `MemoryRepository.Context()` change to:
  ```go
  ctx := domain.NewContext(r, name)
  ctx.Tasks = tasks  // populate after construction
  return ctx, nil
  ```
- The legacy `edit-done` CLI command (`domain.NewContext("done").Filepath()`)
  becomes `ctx, _ := repo().Context("done"); utils.EditFilePath(ctx.Filepath())`.
- TUI's `EditFile` action: already gets its Context from a repo call, no
  change needed.

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

Each method loads any auxiliary context via `c.repo.Context()`, applies the
mutation, and saves through `c.repo.Save(c)`. The snapshot pattern from
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
  ctx, _ := repo().CurrentContext()  // or repo().Context(name) for add-to
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
