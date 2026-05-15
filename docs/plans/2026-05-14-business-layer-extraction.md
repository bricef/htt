# Business Layer Extraction: Domain / Usecase / Storage layering for `htt`

**Date:** 2026-05-14
**Status:** Complete (2026-05-15)

## Goal

Extract a clean business layer with a repository pattern for storage, reused by both the CLI (Cobra) and TUI (Bubble Tea) frontends.

## Current state (summary)

The codebase is **tangled** — no real separation between domain, storage, and presentation:

- **Two duplicate `Context` types**: `internal/todo/context.go` and `internal/core/context.go` (near-identical; `core` looks abandoned).
- **`Sync()` inside domain mutations**: `Context.Add()` and `Context.Remove()` call `Sync()` (file I/O) inline at `internal/todo/context.go:35-50`. Every mutation hits disk.
- **CLI also calls `.Sync()` explicitly**: `internal/cmd/todo.go:34,151,173-176` — sometimes double-syncing.
- **TUI duplicates the pattern**: `internal/interactive/actions.go:119-147`.
- **Global file-based state**: `todo.GetCurrentContext()` / `SetCurrentContext()` at `internal/todo/todo.go:25,33` read/write a single file directly.
- **Errors exit the process**: `utils.DieOnError()` → `os.Exit(1)`.
- **Existing tests**: only `internal/todo/task_test.go` and `internal/todo/parser_test.go` — pure parsing/value tests. Nothing exercises a full use case, CLI, TUI, or storage.

## Critical invariant for this refactor

**No refactor step occurs without a test harness that guarantees behavior remains stable.** Each step pairs (a) tests that pin down current behavior with (b) the structural change. If a step would change behavior without tests covering it, the plan adds those tests as a prior step.

## Pre-existing bug worth knowing about

`Context.Read()` at `internal/todo/context.go:83` calls `c.Add(t)`, and `Add` at line 37 calls `c.Sync()`. **Every read writes the file back.** Since `NewContext` calls `Read`, `todo.NewContext("done")` in TUI/CLI silently rewrites `done.txt` on every invocation.

The harness in Step 1 will likely surface this. The refactor fixes it cleanly in Step 6 (the new `FileRepository.LoadContext` won't write on read). This is the only deliberate behavior change in the plan and should be called out in that commit message.

## Target package layout

```
internal/
  domain/      # Task, Context — pure value types, no I/O
  usecase/     # AddTask, CompleteTask, MoveTask, ... — depend on repo interface
  storage/     # Repository interface + file impl + in-memory impl (for tests)
  cli/         # Cobra commands → usecase
  tui/         # Bubble Tea → usecase
```

---

## Step 1 — End-to-end CLI harness around a temp `$HTT_HOME` (foundation; no refactor)

**Goal:** stand up a black-box test harness that drives the current `htt` binary against a temp data dir and asserts on file contents + stdout.

**Test harness to add:** `test/e2e/cli_test.go`. A `withTempEnv(t)` helper that sets `HOME`/`tracker_path`/`data_path` (or uses Viper test seams via env vars), then drives the binary. Two strategies — pick one and stick to it:
- **In-process**: run `commands.RootCmd` via `RootCmd.SetArgs([...])` and `SetOut(&buf)/SetErr(&buf)`. Blocked today by `utils.DieOnError`'s `os.Exit(1)`.
- **Subprocess**: compile a test binary via `go test -c` or use `go run ./cmd/htt`. Slower but cleaner; works around the `os.Exit` problem.

Table-driven cases for: `todo add`, `todo add-to`, `todo show`, `todo do`, `todo delete`, `todo move`, `todo + / - / priority`, `todo replace`, `todo context`, `todo search`, `todo random` (seeded), `todo status`, and the top-level `status`. Assertions: stdout golden snippets (with `--no-color`) and on-disk `*.txt` contents under `test/e2e/golden/`.

**Refactor change:** none. Pure scaffolding.

**Verify:** all tests green against today's binary, capturing today's behavior (warts included).

**Risk:** the `os.Exit` issue — pick subprocess strategy for v1 to avoid it.

---

## Step 2 — TUI snapshot harness with `teatest`

**Goal:** pin TUI behavior with golden-frame tests before touching it.

**Test harness to add:** `test/tui/tui_test.go`. Add `github.com/charmbracelet/x/exp/teatest` to `go.mod`. Build a fixture: temp data dir prepopulated with two contexts (`todo.txt`, `work.txt`) and a few tasks. Drive `interactive.Model(ctx)` through `teatest.NewTestModel`, send keystrokes (`j`, `k`, `x`, `d`, `+`, `-`, `n` + text + enter, `h`, `l`, `q`), and assert final output via `teatest.RequireEqualOutput` against golden files. Cover at minimum: navigation, add-task, delete, complete, priority +/-, context switch.

**Refactor change:** none.

**Verify:** golden TUI frames captured; tests green.

**Risk:** TUI output contains ANSI color codes — strip via `lipgloss.SetColorProfile(termenv.Ascii)` in setup. If unstable, use fixed `termenv.TrueColor` and accept fixed-color goldens.

---

## Step 3 — Delete dead `internal/core` package

**Goal:** remove the abandoned duplicate Context.

**Test harness to add:** none beyond Step 1+2 (package has zero importers; confirmed unreferenced).

**Refactor change:** delete `internal/core/context.go` and the `internal/core/` directory.

**Verify:** `go build ./...`, `go vet ./...`, all Step 1/2 tests pass.

**Risk:** essentially zero. Cheap win that removes confusion before bigger moves.

---

## Step 4 — Introduce `internal/domain` with pure value types (no behavior change)

**Goal:** create the domain package and move pure value types — `Task`, `Context` struct — without changing any I/O behavior. Methods that touch the filesystem stay where they are for now.

**Test harness to add:** lift `internal/todo/task_test.go` and `parser_test.go` assertions into `internal/domain`; add table-driven tests for `Task.SetPriority/Increase/Decrease`, `Task.Do`, `Task.Annotate/RemoveAnnotation`, `Context.Equals`, `Context.GetTaskById`, `Context.Replace`, `Context.Search`.

**Refactor change:** create `internal/domain/task.go` and `internal/domain/context.go`. Move `Task`, its pure methods, the parser glue, and the `Context` struct (just the type plus pure helpers: `Equals`, `GetTaskById`, `GetTaskByStrId`, `GetTaskIndex`, `Replace`, `Search`, `Sort`). Leave I/O methods (`Read`, `File`, `Filepath`, `Sync`, `Add`, `Remove`, `Show`, `ShowOnly`, `ConsoleString`) in `internal/todo` temporarily. Use a type alias `type Context = domain.Context` in `internal/todo` so existing callers compile unchanged.

**Verify:** Step 1, Step 2, and new domain unit tests all pass.

**Risk:** type aliases for structs with methods defined in two packages can confuse Go method-set rules. Mitigation: keep all methods in `internal/todo` for this step; only the struct fields and the pure functions move. If aliasing causes friction, do this as a rename-import-only change first, then surgically split methods in Step 6.

---

## Step 5 — Define `storage.Repository` interface + in-memory fake

**Goal:** introduce the repository abstraction with no production callers yet — purely additive.

**Test harness to add:** `internal/storage/memory_test.go`. The interface (initial draft):

```go
type Repository interface {
    ListContexts() ([]string, error)
    LoadContext(name string) (*domain.Context, error)
    SaveContext(ctx *domain.Context) error
    GetCurrentContextName() (string, error)
    SetCurrentContextName(name string) error
}
```

Write contract tests in `internal/storage/contract_test.go` that any implementation must pass: round-trip a context, list multiple contexts, current-context get/set, missing-context returns sensible default, save+load preserves task order.

**Refactor change:** create `internal/storage/repository.go` (interface + errors), `internal/storage/memory.go` (map-backed fake), and the contract test. Nothing in `internal/todo`, `internal/cmd`, or `internal/interactive` changes yet.

**Verify:** new contract tests pass; everything else unaffected.

**Risk:** designing the interface too narrowly. Mitigation: pick the smallest set that covers current call sites (above 5 methods is roughly it), expand later if needed.

---

## Step 6 — File-backed `storage.FileRepository`, validated against the same contract

**Goal:** implement the file-backed repo and prove it's behaviorally identical to today's I/O — fixing the read-writes-back bug along the way.

**Test harness to add:** run the same `contract_test.go` suite from Step 5 against `FileRepository` using a temp dir per test. Additionally, golden-file tests: pre-seed a `todo.txt` on disk (with edge cases: blank lines, completed entries, weird priorities), `LoadContext` it, `SaveContext` it back, diff against a golden file to confirm byte-for-byte stability (modulo the well-known `.bak` side effect — assert on that too).

**Refactor change:** create `internal/storage/file.go`. Copy the I/O logic from `internal/todo/context.go` (`Filepath`, `Sync`-equivalent → `SaveContext`, `Read`-equivalent → `LoadContext`) and `internal/todo/todo.go` (`GetCurrentContext`, `SetCurrentContext`, `GetContexts`). **Critically: the new `LoadContext` does NOT call `SaveContext` during read** — this fixes the read-writes-back bug. The contract tests pin the corrected behavior; the e2e tests confirm no externally visible regression.

**Verify:** contract tests pass for both Memory and File; Step 1 e2e tests still pass; document the read-no-longer-writes change in the commit.

**Risk:** subtle file-format drift (trailing newlines, ordering of annotations). Mitigation: golden-file tests with byte-exact assertions on `SaveContext` output before the swap.

---

## Step 7 — Introduce `internal/usecase` with use cases that take a `Repository`

**Goal:** create the business layer as pure functions/methods over the repo interface. Nothing wired in yet.

**Test harness to add:** `internal/usecase/*_test.go`, all using the in-memory repo. Use cases to add (one file each, one test file each): `AddTask`, `AddTaskTo`, `CompleteTask`, `DeleteTask`, `MoveTask`, `ReplaceTask`, `SetPriority`, `IncreasePriority`, `DecreasePriority`, `ListTasks`, `SearchTasks`, `GetCurrentContext`, `SwitchContext`, `ListContexts`. Each test: seed memory repo, invoke use case, assert repo state + returned domain values + returned errors.

**Refactor change:** create `internal/usecase/` package. Implementations are essentially the bodies of the existing CLI commands and TUI actions, but operating on a `Repository` parameter instead of global state, and returning `(result, error)` instead of calling `utils.Fatal`. **No CLI/TUI callers change yet.** This is parallel infrastructure.

**Verify:** new use-case tests pass; e2e and TUI tests unchanged.

**Risk:** designing use cases that don't quite match what CLI/TUI need, forcing rework in Steps 8/9. Mitigation: cross-reference each command in `internal/cmd/todo.go` and each action in `internal/interactive/actions.go` while writing use cases, and write at least one test per existing command alias.

---

## Step 8 — Rewire CLI commands to call use cases (one command group at a time)

**Goal:** swap `internal/cmd/todo.go` from direct domain manipulation to use-case calls. Do this in **3 sub-PRs** — read-only commands first, then mutations, then context switching — each independently merge-able.

**Test harness to add:** none new; Step 1's e2e harness is the safety net. Add an `app.App` struct (`internal/cli/app.go` or similar) carrying the `Repository`, constructed in `main.go` and passed to commands via Cobra's `cmd.SetContext(...)` or a package-level injection. Add unit tests that construct commands with a memory repo and run them via `RootCmd.SetArgs([...])`.

**Refactor change:**
- **8a**: rewire read-only commands (`show`, `status`, `search`, `random`, `context` (read form), `edit-done`) to call use cases.
- **8b**: rewire mutating commands (`add`, `add-to`, `do`, `delete`, `move`, `+`, `-`, `priority`, `replace`).
- **8c**: rewire `context` (switch form), and replace `utils.DieOnError` inside command bodies with `return err` (Cobra propagates) so command logic becomes testable in-process.

After 8c, `internal/cmd/` no longer imports `internal/todo` for mutations — only `internal/usecase` and `internal/domain`.

**Verify:** every Step-1 e2e test still passes at each sub-step. Add in-process Cobra tests with memory repo for fast feedback.

**Risk:** the highest-risk step. `Sync()` semantics, stdout formatting, and error-exit semantics must all be preserved. Mitigation: small sub-PRs, the e2e harness as ground truth, and a moratorium on output-string changes during this step.

---

## Step 9 — Rewire TUI actions to call use cases

**Goal:** `internal/interactive/actions.go` calls `usecase.*` instead of mutating `m.context` and calling `Sync()`.

**Test harness to add:** the Step 2 teatest goldens are the safety net. Add per-action unit tests in `internal/interactive/actions_test.go` that construct a minimal model with a memory repo, call `action.Act(m)`, and assert on the resulting model state and repo state — much faster feedback than teatest for individual actions.

**Refactor change:** thread the `Repository` (or an `app.App`) through `interactive.Model(...)`. Each action becomes: call use case → update model from returned domain values → return `(model, cmd)`. Delete duplicated Add/Remove/Sync logic. Remove `todo.SetCurrentContext` / `todo.GetCurrentContext` calls from `actions.go` in favor of `usecase.SwitchContext`.

**Verify:** Step 2 teatest goldens pass byte-for-byte; new action unit tests pass.

**Risk:** TUI keeps stale `m.context` after a mutation. Mitigation: every mutating action returns a fresh context from the use case and the model assigns it directly, no in-place mutation.

---

## Step 10 — Excise `Sync()`-inside-mutation from the domain

**Goal:** the `Context.Add()` / `Context.Remove()` methods on the domain type no longer touch the filesystem. The repo is the only path to disk.

**Test harness to add:** by this point everything is covered (e2e, teatest, usecase unit, action unit). Add one explicit domain test: `TestContextAddDoesNotTouchDisk` — construct a domain Context with a synthetic Filepath in a temp dir, call `Add`, assert no file was created. This pins the new invariant.

**Refactor change:** delete the `c.Sync()` calls from `domain.Context.Add` (was `internal/todo/context.go:37`) and `domain.Context.Remove` (line 45). Delete the legacy `Sync`, `File`, `Filepath`, `Read` methods from the domain Context entirely (their callers were already migrated in Steps 8 and 9). Delete the legacy `todo.GetCurrentContext` / `todo.SetCurrentContext` / `todo.CompleteTask` / `todo.Move` / `todo.GetContexts` / `todo.ShowStatus` if all callers are migrated; otherwise keep as thin shims that defer to use cases (and mark deprecated).

**Verify:** every test from every prior step passes. The new no-disk-touch test is the explicit anti-regression.

**Risk:** **highest behavioral risk in the whole plan.** Some caller somewhere is implicitly depending on the side effect. Mitigation: this step is small and isolated, and the e2e + teatest goldens are the safety net. If anything fails, revert in isolation.

---

## Step 11 — Rename packages to target layout & collapse `internal/todo`

**Goal:** finalize the target package layout. Cosmetic rename, but it's the moment the architecture becomes obvious to a new reader.

**Test harness to add:** none new — by now everything is well-covered. Maybe add a `internal/architecture_test.go` using `golang.org/x/tools/go/packages` (or a simple `grep` script in `Makefile`) to assert that `internal/domain` imports nothing from `internal/storage`, `internal/usecase`, `internal/cli`, `internal/tui`; and that `internal/cli` and `internal/tui` import `internal/usecase` but not each other.

**Refactor change:** rename `internal/cmd` → `internal/cli`, `internal/interactive` → `internal/tui`. Remove now-empty `internal/todo` (or keep only `parser.go` / `parser_test.go` as `internal/domain/parser.go`). Update imports in `cmd/htt/main.go`.

**Verify:** full test suite + architecture test.

**Risk:** big diff, easy merge conflicts if other work is in flight. Mitigation: do this when no other work is pending. Pure mechanical rename — no behavior changes allowed in this commit.

---

## Step 12 — Make `os.Exit` and stdout injectable in `utils`/CLI seams

**Goal:** remove `os.Exit(1)` from command-logic paths so commands are testable in-process and so `utils.DieOnError` becomes a thin top-level wrapper used only in `main.go` / cobra's `RunE` boundary.

**Test harness to add:** unit tests for command error paths — e.g., `htt todo delete 999` should return a non-nil error from `RunE` rather than `os.Exit`. Assert via Cobra's in-process exec with a captured stderr buffer.

**Refactor change:** convert all `Run:` Cobra handlers to `RunE:` and propagate errors. Move `utils.DieOnError` calls to the outermost boundary (main.go after `RootCmd.Execute()`). Use cases already return `error`; this step just removes the `Fatal` calls that swallowed them.

**Verify:** e2e tests still pass (exit codes preserved for the binary as a whole). New error-path tests pass.

**Risk:** exit-code semantics for the binary change subtly. Mitigation: the Step 1 e2e tests should already assert on exit codes; if they don't, retroactively add that assertion before this step.

---

## Out of scope for this refactor (v1)

Deliberately untouched:

- **`internal/repo` (go-git sync)** — keep as a side-effecting package exposed through a `cli/sync` command and (eventually) a use case. Not blocking the domain/usecase/storage split. Worth wrapping behind a `RemoteSyncer` interface in a future v2, but not now.
- **`internal/timelogs`** — same shape of problem as todos (file I/O mixed with domain). Same refactor pattern applies (timelog domain type + repo). Defer to v2; in v1 it stays as-is and the `workon` command keeps calling it directly.
- **`internal/gcal.go`** — Google Calendar integration; orthogonal, not on any hot path.
- **Viper / Cobra replacement** — keep both. The seams Viper offers via env vars are enough for the test harness.
- **Output / formatting layer** — `ConsoleString()` and color logic stay where they are. They could be extracted into a `presenter` package later, but pulling them out now would balloon the diff in Step 8.
- **Concurrency / file-locking** — the current code has none, the refactor doesn't add any.

---

## Critical files for implementation

- `internal/todo/context.go`
- `internal/todo/todo.go`
- `internal/cmd/todo.go`
- `internal/interactive/actions.go`
- `cmd/htt/main.go`
