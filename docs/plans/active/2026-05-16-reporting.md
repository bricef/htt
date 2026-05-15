# Reporting: "what happened this period?"

**Date:** 2026-05-16
**Status:** Phase 1 complete (2026-05-16); Phases 2 & 3 deferred
**Builds on:** the timelogs refactor (uses `domain.Timelog` and the
`ts:` annotation as the source of truth for time spent)

## Resume marker

Phase 1 (the MVP `htt report` from existing data) is on `main` as
of 2026-05-16. The command groups completions by source context
and prints per-day time totals plus a grand total. Phases 2
(created-on stamping for an "Added" section) and 3 (archive-on-delete
for a "Deleted" section) remain unstarted — they need their own
follow-up plans. Plan stays under `docs/plans/active/` until all
three phases ship.

Phase 1 surfaced one bug worth noting: `domain.NewTask` doesn't
populate `Task.CompletedOn` even when the COMPLETEDAT token is
present in the input. `internal/cli/report.go` works around it with
`completedOnFromRaw` (parses `task.Raw` directly); the parser fix
is captured in `TODO.md` under "Known bugs".

## Motivation

Once `htt` is used daily, the natural Friday-afternoon question is
"what did I get done this week?". Today the user can stare at
`done.txt` and the timelog files, but there's no summary view. A
`htt report` command turns the existing data into a digest.

This plan covers the reporting *initiative* in three sequenced
pieces. Phase 1 is the MVP that works against today's data; phases
2 and 3 unblock the parts of the report that today's data can't
support.

## Decisions captured

These came out of the planning conversation:

1. **Phase 1 ships against existing data.** Completions (via the
   parsed `CompletedOn` on `done.txt` entries) and time spent (via
   `ts:` annotations on timelog entries) need no migration. Ship
   the value first; iterate.

2. **Additions need a separate piece.** Today `Context.AddTask`
   doesn't stamp a created-on date even though the todo.txt parser
   already understands the `CreatedOn` field. Phase 2 adds that
   stamp; Phase 1's report has no "Added" section.

3. **Deletions need a design call, not just a feature.** Today
   `Context.Delete` removes the task from disk entirely — no audit
   trail. The fix is the long-standing TODO "Change delete from
   removal to archival in separate file". Phase 3 is its own plan
   doc because the design choice (separate `archived.txt` per
   context? a top-level archive log?) matters more than the code.

4. **Time totals exclude the still-active entry.** `Timelog.Spans()`
   only counts pairs of consecutive entries. The trailing entry has
   no closing span. The report says "Total: 28h 45m (excluding
   currently-active entry)" so the user knows. `htt log status`
   stays the way to see what's running right now. This matches the
   timelogs refactor's "Latest returns @end as the active entry"
   decision: we preserve the naive semantic and let the user
   reconcile.

5. **`--since` accepts a date or a shorthand duration.**
   `htt report --since 2026-05-09` (absolute) or
   `htt report --since 7d` / `--since 2w` (relative). Default 7d.
   `time.ParseDuration` doesn't handle days/weeks, so the parser is
   custom. Months and years are deferred — they're calendar-aware
   and ambiguous in ways that matter for reports.

6. **The reporting code lives in `internal/cli/report.go`.** No new
   package yet. The aggregation logic (filtering done.txt by date,
   iterating timelogs across a range, summing spans) is small
   enough to live alongside the CLI command. Extract later if it
   grows.

## Shape

```
$ htt report
Activity since 2026-05-09 (7 days)

Completed (5)
  work:
    (A) ship the auth refactor             2026-05-13
    review PR #4123                        2026-05-12
    update on-call runbook                 2026-05-11
  home:
    pay credit card                        2026-05-10
    weekly meal plan                       2026-05-09

Time logged
  2026-05-09  4h 30m
  2026-05-10  6h 15m
  2026-05-11  5h 50m
  2026-05-12  7h 10m
  2026-05-13  5h 00m
  Total: 28h 45m (excluding currently-active entry)
```

Flags:

- `--since` — date `YYYY-MM-DD` or shorthand `Nd`/`Nw`/`Nh`.
  Default `7d`.
- `--until` — same shape; default `now`. Hidden for MVP; ships if
  needed in iteration.

Domain additions (Phase 1):

```go
// internal/domain/timelog.go

// Span pairs a timelog entry with the duration the user spent on it
// before the next entry was appended. The final entry has no span —
// it's still in progress.
type Span struct {
    Entry    *Task
    Duration time.Duration
}

// Spans walks consecutive Entries and computes the time between
// each. Returns an empty slice for timelogs with fewer than two
// entries. Returns an error if any entry's ts: annotation is
// missing or malformed.
func (l *Timelog) Spans() ([]Span, error)
```

No repository surface change in Phase 1. The report iterates dates
with the existing `Day(date)` method.

## Steps

### Phase 1 — `htt report` over existing data

Each commit keeps `just test` green.

**Step 1.1 — `domain.Span` + `Timelog.Spans()`**

Test harness: white-box tests in `internal/domain/timelog_test.go`
covering empty timelog (no spans), single-entry (no spans),
two-entry (one span), three-entry (two consecutive spans), and the
missing/malformed-ts error path.

Refactor: add the type and method to `internal/domain/timelog.go`.
Pure function, no `time.Now()` dependency, no repo dependency.

**Step 1.2 — `htt report` command**

Test harness: in-process Cobra test in `internal/cli/todo_test.go`
seeding a memory repo + memory timelog repo, running
`runCobra(t, "report", "--since", "30d")`, and asserting no error.
Output assertions are weight-bearing once the format settles; for
MVP the smoke (no error, expected sections) is enough.

Refactor: new file `internal/cli/report.go`:

- `parseSince` helper accepting `YYYY-MM-DD` or `Nd`/`Nw`/`Nh`.
- `Report` cobra command, registered on `RootCmd`.
- Section 1: load `done` context, filter by `CompletedOn` in range,
  group by `Annotations["context"]`.
- Section 2: iterate dates from `since` to `now`, load each
  `Timelog` via `timelogRepo().Day(date)`, sum `Spans()`. Print
  per-day total plus grand total.

**Verify:** `just test` green; manual smoke (`htt report` on a real
data dir) shows the expected sections.

**Risk:** the timelog parser's handling of RFC3339 in `ts:` values
is load-bearing. The earlier timelogs work confirmed it survives
parse round-trips, but the report is the first caller that does
arithmetic on them.

### Phase 2 — Created-on stamping (separate session)

`Context.AddTask` annotates new tasks with `CreatedOn = time.Now()`
(the parser already populates `CreatedOn` from the `YYYY-MM-DD`
prefix; we just have to set it on creation). Report grows an
"Added" section that filters by `CreatedOn` in the same range.

One-line domain change, one CLI section. Maybe a config flag if a
user doesn't want the date stamp on every task — but default on.

### Phase 3 — Archive-on-delete (own plan doc)

Out of scope here. The choice between an archive file per context,
a single rolling archive, or a separate archive directory deserves
its own design dialogue. The reporting "Deleted" section ships
when that plan lands.

## Out of scope

- Calendar week / month modes (`--week`, `--month`). The
  shorthand `7d` covers the common Friday-review case; calendar
  modes can ride in when the user wants them.
- Time-per-task aggregation. Spans give us this, but grouping by
  task text gets messy when the same activity gets logged with
  slightly different wording. Defer.
- HTML / JSON output. The terminal output is what the user
  actually looks at.
- Editing reports (e.g. correcting a forgotten `log end`). The
  existing `htt log edit` already opens the file; users can fix
  by hand.
