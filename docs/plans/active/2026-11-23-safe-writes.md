# Safe writes: building the agreed design

**Date:** 2026-11-23
**Status:** In progress

The design is [docs/design/safe-writes.md](../../design/safe-writes.md)
(agreed 2026-11-23, gofrs/flock for the lock). This plan doesn't restate
it: it says how it's built, in which stages, how it's tested, and the
choices the design leaves open.

## Stages

Each stage is a commit that builds and passes `just test`, `just vet`
and `just check`.

1. **The tests that fail today**, and **the mechanism**: the
   concurrency tests below, seen failing on main; then
   `internal/storage/transaction.go` (the transaction, the lock, the
   check, the atomic replacement), with `FileRepository.Save`,
   `SetCurrent` and `FileTimelogRepository.Save` writing through it,
   which ends torn writes.
2. **The domain's save point and the multi-file operations**:
   `Repository.Transact`; every persistent `Context` method runs as
   one transaction on the content as it is now; the memory repository
   implements it, with its seam. This ends lost updates between htt
   processes.
3. **Time logs and the pointer**: `TimelogRepository.Transact`,
   `Timelog.Append` and `AppendAt` as transactions; the pointer's
   write locked and atomic (stage 1 already routes it).
4. **The TUI**: its reload-and-find folded into the transaction
   (`Context.Transact`); watching the file it shows.
5. **Docs**: GETTING-STARTED, architecture, TODO, the design's status
   line; this plan to `complete/` with an outcome.

## Choices

### The abstraction

- **`domain.Repository.Transact(change func(tx Repository) error) error`**
  and **`domain.TimelogRepository.Transact(change func(tx
  TimelogRepository) error) error`**. `tx` is a repository scoped to
  the transaction: its `Context` (and `ContextLines`, `Day`, …) reads
  the store as it is now, its `Save` and `SetCurrent` hold what
  they're given until `change` returns, and `Transact` on it runs the
  change as part of the same transaction. When `change` returns nil
  and nothing it read has changed, the saves are made in the order
  `change` made them. The domain already saves the file gaining tasks
  first (Complete, Delete, Move), so the storage keeps that order
  rather than guessing which file gains.
- **Errors the caller can see**, declared by the domain (the contract)
  and returned by storage: `domain.ErrBusy` ("another htt is busy with
  your files; try again"), `domain.ErrKeepsChanging` (wrapped as
  "<file> keeps changing; nothing was saved"), and
  `domain.ErrTaskChanged` for an operation that can't find its task
  in the content as it is now.
- **One file.** `internal/storage/transaction.go` holds the
  transaction (read once, recorded; writes held, in order; the check;
  the retries), the lock (gofrs/flock v0.13.2, the newest release at
  least two weeks old: 2026-11-08) and the atomic replacement. The
  repositories' transaction views (`file.go`, `timelog_file.go`) only
  parse what the transaction reads and serialise what they save;
  nothing else in htt touches the lock or a temporary file, which an
  architecture test pins.

### The domain

- **The save point.** Every persistent `Context` method runs its body
  through one helper, which runs it as a transaction of the context's
  repository on the context as the store holds it then: the context's
  tasks become the file's (its own task objects when they're the same
  lines, so a task the caller holds sees the change, as now), and its
  repository is the transaction's while the body runs, so what the
  body loads (done, the archive, the move's target) and saves is part
  of the transaction. Afterwards the caller's `Context` holds the
  result. A body may run more than once; each run starts from the
  store, and a task the caller passed in is put back as it was given
  first (a subtask's `p:` is chosen afresh).
- **Task numbers** name the task the caller's context had at that
  number when the operation was called; run on the content as it is
  now, the operation finds that task by its line (the design's
  "complete the task whose line is …"): where it was, or the one task
  with that line, or it refuses with `ErrTaskChanged`. For a CLI
  command that's line N as the command read it, milliseconds before,
  as now; for `todo edit` and `todo do`'s question, which wait for the
  user, it's the task the user saw, not whatever is at N by then.
- **Composing.** `Context.Transact(body func(c *Context) error)` runs
  several steps as one transaction on the context as it is now, with
  no renumbering: the body finds what it acts on in `c`. Persistent
  methods called inside join it. The TUI uses it to find the task it
  showed by its text and act on it in one transaction.
- **Time logs** follow the same pattern (`Timelog.Append`, `AppendAt`).

### Storage

- **Locks:** `.htt.lock` in each directory a transaction can write: a
  `FileRepository` takes its data directory's and its pointer
  directory's (one lock when they're the same), a
  `FileTimelogRepository` its log directory's; several are taken in
  sorted order. The name doesn't end in `.txt` or `.log`, so it's
  never a context or a day. The wait is bounded (5 s: a completion
  parses the whole done file, as it does today, which holds the lock
  about a second at a few thousand done tasks), then `ErrBusy`. The
  bound is a package variable that storage's tests shorten.
- **The check:** the bytes read are kept, and compared with the file's
  bytes under the lock before anything is written (a file that didn't
  exist must still not exist). On a difference the change runs again,
  up to 5 runs in all, then `ErrKeepsChanging`, with nothing written.
- **Atomic replacement:** a temporary file beside the target
  (`.<name>.<random>.tmp`, created exclusively with the files' mode as
  now, so the umask applies as it did), written, synced, closed, and
  renamed over the target. The `.bak` is still the previous version:
  it's written from the bytes read (checked to be the file's), the
  same way, before the file is replaced. On Windows `os.Rename` is
  `MoveFileEx` with `MOVEFILE_REPLACE_EXISTING`, which replaces an
  existing file, but fails while another process has the file open
  (Go opens files without `FILE_SHARE_DELETE`), so there a rename that
  fails with access denied or a sharing violation is retried for up
  to two seconds.
- **Save and SetCurrent outside a transaction** are transactions of
  one write (locked, atomic). Reads outside a transaction take no
  lock: atomic replacement means they see a whole file.
- **The memory repositories** copy tasks on load and save (as the
  memory time-log repository already does), implement `Transact`
  with held saves and the same retry count, and have an `Underneath`
  hook, run between a change and its check, for tests to change the
  store underneath. No lock.
- **Seams for storage's own tests**: unexported hooks on the file
  repositories, run between the change and the check, and before each
  replacement (to fail one).

### The TUI

- Each action that changes tasks runs inside `Context.Transact`: find
  the task shown, by its text (`shownAt`, as now), then act; the
  checks it makes (the task above for Tab, the open subtasks for the
  answer to x) run inside too. When the task isn't there, the status
  line says so, as now.
- **Watching:** a tick each second stats the shown file
  (`ContextPath`): another file (`os.SameFile`, which an atomic
  replacement changes), size or modification time differing from the
  last look reloads the list by name and redraws it, the selection on
  the same task by its text where it's still there. No new dependency;
  nothing is watched without a path (the memory repository).

## Testing

Data safety is the bar; tests first, seen failing on main where they
can.

- **Lost updates (e2e):** N real htt processes add to one context at
  once; all N must land. Bounded: N = 20, a deadline per process.
  Also real `todo add`s racing the TUI's actions (the real model, on
  files) in the TUI's tests.
- **Torn writes:** storage-level (a writer saving repeatedly, a reader
  reading in a loop) and e2e (htt rewriting a file repeatedly, the
  test reading it): every read is a whole old or new version, never
  missing or partial. Bounded in count and time.
- **Changes underneath:** through the seams, a change between read and
  check makes the transaction run again, keeping both changes; a file
  changing on every run gives `ErrKeepsChanging` with nothing written.
  The same with the memory repository's `Underneath`, for the domain's
  operations (re-run on the new content, by the task's line).
- **Lock timeout:** a lock held elsewhere gives `ErrBusy` within the
  (shortened) bound; e2e, the message, with the file untouched.
- **Several files:** the gaining file is written first; a failure
  between the two writes leaves the task in both files, never in
  neither.
- **Encapsulation:** the architecture test still passes; a new one
  checks that only `internal/storage/transaction.go` imports the lock
  library or writes, renames or creates files in storage.
- **Windows:** CI runs these. Tests that can't run there say why.
- **Mutations**, each tried by hand and the catching test noted in the
  outcome: no lock; no check; writing in place; the losing file
  first; no retry; no `.bak`.
- `go test -race` on storage, domain and tui.
