# `htt todo show --sort / --filter`

**Date:** 2026-05-16
**Status:** Proposed

## Motivation

Today `htt todo show` prints tasks in file order. The only
filtering is `htt todo search <regex>`; the only sorting is a
priority sort triggered as a side-effect of priority mutations
(via `Context.Sort`). There's no on-demand way to ask "show me
the oldest task in this context" or "what's due this week".

The data is already parsed — `CreatedOn`, `Annotations["due"]`,
`Priority`, `Tags` are populated on every Task — it just isn't
exposed as a query axis. Adding `--sort` and `--filter` flags to
`show` turns those fields into useful daily views.

Ship the most-asked-for axes (created, due) plus a few cheap
extras (priority, overdue, tag), iterate based on real use.

## Decisions captured

From the planning dialogue (2026-05-16):

1. **Shape: flags on `show`, not new subcommands.** Composable
   (`--sort X --filter Y`), keeps the surface area small, plays
   nicely inside the REPL (`show --sort created` works there too
   without any REPL-specific handling). Subcommand sprawl
   (`list-by-due`, `list-overdue`) was rejected.

2. **Default behaviour unchanged.** `htt todo show` with no flags
   produces the same output as today (file order, no filter,
   `(name): N tasks` footer). The flagged path is purely additive.

3. **Sort keys for v1: `priority`, `created`, `due`.** A `-`
   prefix reverses direction (`--sort -created` → newest first).
   `entry` (alphabetical) is omitted — easy to add later if a
   user reaches for it.

4. **Filters for v1: `overdue`, `due-soon`, `no-due`,
   `has-tag:<x>`, `priority:<X>`.** `no-priority` and other axes
   are easy to add later but excluded from v1 to keep scope
   tight. `--filter` is repeatable (StringSlice) with implicit
   AND semantics.

5. **Display preserves original task indices.** Even when the
   view is filtered or sorted, each line shows the task's
   `Line` field (its original file index) — so `delete 7` still
   targets the right task regardless of where it appears in
   the sorted list.

6. **Footer changes only for the flagged path.** Default path
   keeps `(name): N tasks`; flagged path shows
   `(name): N of M tasks shown` so the user knows the view is
   not the full list.

7. **Empty result has a dedicated message.** A filter that
   matches nothing prints `(name): No tasks matched query.` —
   same line `Context.ShowOnly` already uses for search.

## Shape

### Sort keys

| Key             | Order                                                              |
| --------------- | ------------------------------------------------------------------ |
| `priority`      | A < B < C < no-priority. Same convention as `Context.Sort`.        |
| `created`       | Oldest `CreatedOn` first. Tasks without `CreatedOn` go last.       |
| `due`           | Soonest `due:` first. Tasks without `due:` go last.                |
| `-<key>`        | Reverse order. `-created` = newest first; `-due` = furthest first. |

Tasks tied on the primary key keep their original relative order
(stable sort).

### Filter expressions

| Expression          | Predicate                                                              |
| ------------------- | ---------------------------------------------------------------------- |
| `overdue`           | Has `due:`, parsed date is before today's local midnight.              |
| `due-soon`          | Has `due:`, parsed date is in `[today, today+7d)`.                     |
| `no-due`            | No `due:` annotation.                                                  |
| `has-tag:<name>`    | Any of the `@<name>`, `+<name>`, `#<name>` tags is present.            |
| `priority:<letter>` | `Priority == strings.ToUpper(letter)`. Reject anything outside A-C.   |

Repeatable `--filter` ANDs all predicates. An unknown filter
expression errors with `invalid --filter "<expr>"`.

### Example sessions

```
$ htt todo show --sort -created
(work)
  3 (B) wrap presents
  1 review PR for the new auth flow
  0 ship the auth refactor
(work): 3 tasks

$ htt todo show --filter overdue --sort due
(work)
  4 (A) submit timesheet                            due:2026-05-14
  1 review PR for the new auth flow                 due:2026-05-15
(work): 2 of 5 tasks shown.

$ htt todo show --filter has-tag:home
(work)
  2 buy milk @home
  5 fix the sink @home
(work): 2 of 5 tasks shown.

$ htt todo show --filter priority:A --filter due-soon
(work)
  4 (A) submit timesheet                            due:2026-05-17
(work): 1 of 5 tasks shown.
```

### File layout

New file `internal/cli/show.go` holding:

- `var show = &cobra.Command{...}` — moved from `todo.go`.
- Flag globals: `showSortKey string`, `showFilters []string`.
- `parseSortKey(key string) (less func(a, b *Task) bool, error)`.
- `parseFilter(expr string) (predicate func(*Task) bool, error)`.
- Comparator helpers: `lessByPriority`, `lessByCreated`,
  `lessByDue` (uses the same `parseAnnotationDate` already in
  `report.go`).
- Predicate helpers for each filter expression.
- The flagged-path runner that filters → sorts → prints with the
  modified footer.

`internal/cli/todo.go` loses the `show` var. The
`TodoCommand.AddCommand(show)` line in `todo.go`'s `init` stays
(still references the package-level `show`).

`internal/cli/repl.go`'s `resetReplFlags` grows entries for the
two new globals so REPL cycles can't leak flag state.

### Reused helpers

- `parseAnnotationDate` (in `report.go`) — parse `due:` value as
  local-date.
- `startOfDay` (in `report.go`) — anchor for `overdue` /
  `due-soon` comparisons.
- `Context.Search(predicate)` — single-predicate filter; chain
  multiple predicates into one closure for AND semantics.

## Steps

One commit. Tests where they buy real coverage.

### Step 1 — `internal/cli/show.go`

1. New file; declare `show` Cobra command + flag bindings in
   `init()` (use `StringVar` for `--sort`, `StringSliceVar` for
   `--filter`).
2. Move the show-body from `todo.go` (revert that one-line
   removal will already have created clean state).
3. Implement `parseSortKey`, `parseFilter`, comparators,
   predicates, the flagged-path runner.

### Step 2 — Update `resetReplFlags`

Add `showSortKey = ""; showFilters = nil` to the reset list in
`internal/cli/repl.go`. Update the existing
`TestResetReplFlags_ClearsBleedingState` to seed these too.

### Step 3 — Tests

New `internal/cli/show_test.go` with three groups:

- **Comparators (table tests).** For each `(less func, input
  tasks, want order)` triple: assert sort.SliceStable produces
  the expected order. Cover priority, created, due, and the
  "missing field goes last" edge cases.

- **Filter predicates (table tests).** For each filter
  expression, build a small set of representative tasks and
  assert the predicate matches the expected subset. Cover the
  unknown-filter error path.

- **CLI integration.** Seed a memory repo with a handful of
  tasks (mixed priorities, mixed `CreatedOn`, mixed `due:`),
  run `runCobra(t, "todo", "show", "--sort", "-created")` and
  assert no error. Stdout isn't captured here — the smoke
  proves the flagged path walks the data without errors. A
  separate small test for the empty-result path.

### Step 4 — Docs

A new short section in `GETTING-STARTED.md` after the existing
"Your first task" / priorities sections covering `--sort` /
`--filter` with the example session shapes above.

## Out of scope

- **Multi-key sort** (`--sort "priority,-created"`). Useful but
  YAGNI for v1.
- **Negated filters** (`--filter !overdue`, `--filter
  not:has-tag:home`). Skip until needed.
- **Date-range filter on `created`.** `due-soon` covers the
  immediate need; a generalised `created:since:<expr>` is more
  work than its likely use justifies.
- **TUI surfacing.** The TUI doesn't get sort/filter UI here;
  it stays a separate workstream if ever needed. Adding the
  capability to `Context.Search` predicates is enough to wire
  it later.
- **Persisted views.** No `htt todo view save <name>` yet.
  Iterate after seeing what filters/sort combos get reached for
  daily.

## Risks

- **Default-path regression.** The most likely accidental break
  is changing `htt todo show`'s output when no flags are passed.
  Mitigation: keep the unflagged path running the existing
  `ctx.Show()` exactly as today; the new path is an
  early-branch within RunE only when `showSortKey != "" ||
  len(showFilters) > 0`.
- **Index-after-sort confusion.** Showing `Line` from the
  original file order means a sorted view looks "out of order"
  by index. Documented in `--help` output and the
  GETTING-STARTED section. Trade-off accepted because the
  alternative — renumbering — would break `delete <N>`.
