# Archive-on-delete (reporting Phase 3)

**Date:** 2026-05-16
**Status:** Complete (2026-05-16)
**Builds on:** the reporting plan at
[`2026-05-16-reporting.md`](2026-05-16-reporting.md). Phase 3 of the
reporting initiative; both plans land together.

## Motivation

`Context.Delete` removes the indexed task from the on-disk file and
that's it — there is no audit trail. Long-standing wishlist item
"Change delete from removal to archival in separate file" calls
this out. The reporting feature also wants a "Deleted" section
alongside Completed and Added, which needs somewhere to look for
the deletion history.

Solving both together: re-target `Context.Delete` so the task lands
in an archive bucket with two annotations capturing source and
deletion date.

## Decisions captured

These came out of the planning dialogue (2026-05-16):

1. **Single archive bucket for all contexts.** One file at
   `<dataDir>/archive.txt`, modelled as a reserved context named
   `archive` (sibling to `done`). One file beats per-context
   archives because the user doesn't typically slice deletions
   by source — and the report can group on the fly via the source
   annotation. Single line entries don't take significant space
   unless someone runs `htt` daily for decades.

2. **Manual garbage collection only.** No auto-prune. Users can
   open `archive.txt` and delete lines by hand when they want to.
   `htt todo edit-archive` opens the file in `$EDITOR` (mirror of
   `htt todo edit-done`).

3. **`archived-from:<srcContext>` annotation, distinct from
   `context:`.** `context:` is already used by `done.txt` for
   "completed in this context". Reusing it on delete would overwrite
   the completion-source annotation when a `done.txt` entry is
   deleted — minor info loss for an edge case but worth avoiding.
   `archived-from:` makes the semantic explicit and never collides.

4. **`deleted-on:YYYY-MM-DD` annotation** for the deletion date.
   Used by the report's "Deleted" section to filter by period.
   Date-only (no time-of-day) keeps it consistent with
   `CompletedOn` / `CreatedOn` and matches what the report
   needs.

5. **No structural marker for deleted tasks.** done.txt prefixes
   `x DATE` on each entry; archive.txt has no analog. Annotations
   are enough signal — the file is identified by its name, not by
   per-line markers. Keeping the format that way means archive
   entries parse via the existing grammar with no parser changes.

6. **The original Raw is preserved.** A task with `CreatedOn`,
   `Priority`, or a completion-context annotation keeps all of
   those when archived. Only the two new annotations are appended.

7. **`archive` is excluded from switchable contexts** (same as
   `done`). `SwitchableContextNames` filters both out, so it
   doesn't appear in tab strips, status output, or the TUI context
   bar. Reachable only via `htt report`, `htt todo edit-archive`,
   or by editing the file directly.

## Shape

### A deleted task in archive.txt

Original task in `todo.txt`:

```
2026-05-10 (A) draft the migration plan
```

After `htt todo delete 0` on 2026-05-16:

```
# archive.txt
2026-05-10 (A) draft the migration plan archived-from:todo deleted-on:2026-05-16
```

The original `CreatedOn` and priority are preserved. Two new
annotations capture the where and when.

For a task deleted from `done.txt`:

```
# done.txt before
x 2026-05-15 2026-05-10 (A) the thing context:work

# done.txt after delete
(line removed)

# archive.txt after delete
x 2026-05-15 2026-05-10 (A) the thing context:work archived-from:done deleted-on:2026-05-16
```

The `context:work` from completion is preserved; `archived-from:`
captures that the *source of the delete* was `done`, not `work`.

### Report output (with Deleted section)

```
$ htt report
Activity since 2026-05-09 (7 days)

Completed (5)
  work:
    (A) ship the auth refactor             2026-05-13
    ...

Added (4)
  work:
    ...

Deleted (2)
  todo:
    (A) draft the migration plan           2026-05-16
  done:
    the thing                              2026-05-16

Time logged
  ...
```

Grouping is by `archived-from:` (the source context), mirroring
how Completed groups by `context:`.

### Domain additions

```go
// internal/domain/repository.go
const ArchiveContextName = "archive"
```

`SwitchableContextNames` filters both `done` and `archive`.

```go
// internal/domain/task.go (unexported, sibling of markCompleted)
func (t *Task) markDeleted(source *Context, when time.Time) (*Task, error) {
    if _, err := t.Annotate("archived-from", source.Name); err != nil { ... }
    if _, err := t.Annotate("deleted-on", when.Format("2006-01-02")); err != nil { ... }
    return t, nil
}
```

```go
// internal/domain/context_ops.go
func (c *Context) Delete(strID string) (*Task, error) {
    target, err := c.GetTaskByStrId(strID)
    if err != nil { return nil, err }
    if c.Name == ArchiveContextName {
        // Deleting from the archive itself is true removal — the
        // current behavior. Otherwise we'd loop forever (move into
        // the same file, annotate again, never actually drop).
        if err := c.remove(target); err != nil { return nil, err }
        return target, c.repo.Save(c)
    }
    archive, err := c.repo.Context(ArchiveContextName)
    if err != nil { return nil, err }
    if _, err := target.markDeleted(c, time.Now()); err != nil { return nil, err }
    if err := c.remove(target); err != nil { return nil, err }
    archive.add(target)
    // Destination-first save (same reasoning as Move / Complete):
    // partial failure errs toward duplication, not loss.
    if err := c.repo.Save(archive); err != nil { return nil, err }
    if err := c.repo.Save(c); err != nil { return nil, err }
    return target, nil
}
```

### CLI additions

```go
// internal/cli/todo.go
var editArchive = &cobra.Command{
    Use: "edit-archive",
    Aliases: []string{"ea"},
    ...
}
```

```go
// internal/cli/report.go
func reportDeleted(since, until time.Time) error {
    archive, err := repo().Context(domain.ArchiveContextName)
    ...
    for _, task := range archive.Tasks {
        deletedOn, ok := parseAnnotationDate(task, "deleted-on")
        if !ok { continue }
        // filter by [sinceDay, until], group by archived-from:
    }
}
```

Plugged into `Report.RunE` after Added, before Time logged.

## Steps

Each commit keeps `just test` green.

### Step 3.1 — `ArchiveContextName` + `SwitchableContextNames` filter

Test harness: an existing-style test in `internal/domain/repository_test.go`
(or wherever `SwitchableContextNames` is currently tested) verifying
that a repo with `archive` and `done` and `todo` and `work` returns
`[todo, work]` from `SwitchableContextNames`.

Change: add the constant; extend the filter to skip both reserved
names.

### Step 3.2 — `Context.Delete` archives instead of removing

Test harness in `internal/domain/context_ops_test.go`:

- `TestContext_Delete_MovesToArchive`: a fresh todo + delete →
  todo empty, archive holds one task with `archived-from:todo` and
  `deleted-on:YYYY-MM-DD`.
- `TestContext_Delete_PreservesOriginalAnnotations`: a task with a
  `context:work` annotation (e.g. from done) survives the delete
  with both `context:work` AND `archived-from:done`.
- `TestContext_Delete_FromArchiveIsTrueRemoval`: a task already in
  the archive gets fully removed (no recursive re-archive).
- `TestContext_Delete_SavesArchiveFirst`: order-tracking repo
  pinning destination-first save semantics (bug_015 reasoning).

Update the in-tree test that asserts post-delete the task is "gone"
from todo — it should now check both that todo no longer has it AND
that archive does.

Change: rewrite `Context.Delete` per the shape above; add unexported
`Task.markDeleted` helper alongside `markCompleted`.

### Step 3.3 — `htt report` Deleted section

Test harness: a `runCobra(t, "report", "--since", "30d")` smoke test
that seeds an archive context and asserts the Deleted line shows
the expected count.

Change: add `reportDeleted` to `internal/cli/report.go`. Iterates
the archive context, parses each entry's `deleted-on:` annotation,
filters by [sinceDay, until], groups by `archived-from:`, prints in
the same shape as Completed / Added.

Annotation date parsing helper: `time.ParseInLocation("2006-01-02", ..., time.Local)`.
Skip entries with missing or malformed `deleted-on:` (don't error
the whole report).

### Step 3.4 — `htt todo edit-archive`

Test harness: not really needed beyond compile-success. The command
shells out to `$EDITOR`; the existing `edit-done` has no test and
the path resolution is shared with that.

Change: add a Cobra subcommand mirroring `editDone` in
`internal/cli/todo.go`.

### Step 3.5 — Plan + TODO cleanup

- Move `2026-05-16-reporting.md` to `docs/plans/complete/` (now that
  the initiative is done).
- Move this plan to `docs/plans/complete/`.
- Drop the "Change delete from removal to archival in separate file"
  line from `TODO.md` (delivered).

## Out of scope

- **`htt todo restore <archive-id>`.** Recovering an accidentally
  deleted task by ID. Plausible follow-up but not required for the
  reporting initiative — the user can copy lines out of `archive.txt`
  by hand for now.
- **Backfill of pre-feature deletions.** Existing installs have no
  archive history. The Deleted section starts empty and accumulates
  from here.
- **Auto-prune.** Decided manual. Revisit only if the file genuinely
  becomes a problem in practice (it won't).
- **Per-context archives.** Single bucket only. Revisit if reporting
  needs scoped views like "deletions from todo last week" — easy to
  add later by walking the file and filtering by `archived-from:`.
