# Safe writes

**Status:** Agreed, 2026-11-23 (Brice), with gofrs/flock for the lock.
Built 2026-11-23, on `feat/safe-writes`.

htt's files are plain text, and more than one thing writes them: two htt
commands at once (a script, or the CLI beside the TUI), the TUI over
time, your editor, topydo, a sync tool. This document says how htt keeps
every change when that happens, and how that concern stays in one place
so the rest of htt doesn't have to think about it. How it's built and
tested is planned separately.

## The problem

There are three ways things go wrong today.

1. **Torn writes.** To save a file, htt renames it to `.bak`, then
   creates the file again and writes the tasks into it. For that moment
   the file is missing, then half-written. Another htt reading it then
   sees an empty or partial list, and its own save writes that back:
   most of a file can go.
2. **Lost updates.** Two writers each read a file, change their copy and
   write it back; the second write discards the first one's change. The
   TUI is the clearest case: it held its list for as long as it was
   open, so a task added from the CLI was gone after the next change in
   the TUI. Since v1.1 the TUI reads the file again before each change,
   which shrinks the window to milliseconds; it doesn't close it, and
   two CLI commands have the same window.
3. **Stale views.** The TUI shows the list as it was when it last read
   it. Since the fix in 2 this can't lose anything, but you don't see a
   task the CLI added until you act.

## Goals

- **Between htt processes, nothing is ever lost.** Any number of htt
  commands, REPLs and TUIs can change the same files at once.
- **Against other tools, as safe as plain files allow.** An editor or
  topydo doesn't know htt's rules, so htt can only make the window in
  which their change and htt's collide tiny, and notice most
  collisions.
- **A reader never sees a missing or half-written file,** whatever
  writes it.
- **One place.** The mechanism is written once, in the storage layer,
  and every write in htt goes through it without the calling code
  knowing. Callers see it only when something can't be resolved.

Not goals: a database, a background service that owns the files, or
anything that stops htt's files being todo.txt files you can edit.

## How others do it

| Tool | Approach | Solves |
| --- | --- | --- |
| git | A `.lock` file, created exclusively, holds the new content; renaming it over the real file commits it | Torn writes and lost updates between git processes; a crash can leave a stale lock to remove by hand |
| Taskwarrior 2 | Operating-system locks on its data files (its `locking` setting, on by default) | Lost updates between `task` processes |
| Taskwarrior 3, most apps | SQLite: real transactions | Everything, but the data stops being plain text |
| Vim, VS Code | Check the file's timestamp before saving; say it changed since it was read | Lost updates against anyone, by asking |
| Most careful writers | Write a temporary file, then rename it over the original | Torn writes |
| Editors, file managers | Watch files and reload when they change | Stale views |

htt combines the four that fit plain files: atomic replacement, an
operating-system lock between htt processes, a check before replacing,
and watching for the view.

## Design

### One mechanism: the transaction

Every change htt makes to its files runs as a **transaction**: a
function that, given the files as they are now, makes its change. The
storage layer runs it:

1. **Lock.** Take htt's lock (below), waiting if another htt holds it.
2. **Read** the files the change needs, fresh from disk, noting each
   one's content as read.
3. **Change:** run the function on what was read.
4. **Check** that no file it read has changed on disk since (another
   tool wrote it). If one has, go back to 2 and run the function again
   on the new content; after a few attempts, give up with an error.
5. **Write** each changed file atomically: the new content goes to a
   temporary file beside it, which is then renamed over the file, so
   any reader sees the old file or the new one and never anything in
   between. The previous version is kept as `.bak`, as now.
6. **Unlock.**

The function describes intent ("complete the task whose line is …",
"add this task"), not a finished list, which is what lets the
transaction run it again on fresher content. A change that touches
several files (completing moves a task to the done file; deleting, to
the archive; moving, to another context) runs as one transaction, and
writes the file gaining tasks before the one losing them, so a crash
part-way can leave a task in both places but never in neither.

### The lock

htt takes an operating-system advisory lock on a lock file in each
directory it writes (the data directory, and the time-log directory
when it's elsewhere), always in the same order, so two transactions
can't wait on each other. An operating-system lock is released when
its process ends, so a crashed htt never leaves a lock behind, unlike
git's lock files. Every htt process takes it for every write, which
serialises htt's changes; transactions take milliseconds, so the wait
isn't noticeable. Other tools don't take it, which is what step 4's
check is for.

### Where it lives

- **The mechanism** (transaction, lock, check, atomic replacement) is
  one file in the storage layer. Go's standard library has no portable
  file lock, so the lock itself comes from gofrs/flock, a small,
  maintained library over `flock` on Unix and `LockFileEx` on Windows,
  which keeps the mechanism to that one file.
- **The domain** already sends every persistent change through one
  point (a context's save, a time log's save). That point becomes "run
  this operation as a transaction", so the domain's operations (add,
  complete, delete, move, nest, set priority, log an entry, …) are
  transactions by construction.
- **Callers don't change.** The CLI, REPL, TUI and GUI call the same
  operations as now and never see a lock, a retry or a temporary file.

### Covered writes

The context files, the done file, the archive, the time logs and the
current-context pointer. The update check's cache already writes
atomically and needs no lock; the first-run config file is written once.

### What a caller can see

Only what can't be resolved:

- **The task is gone or changed.** The operation, run on fresh content,
  can't find what it was asked to act on (the TUI already says this on
  its status line).
- **htt couldn't get the lock** within a few seconds: "another htt is
  busy with your files; try again".
- **A file kept changing** under several attempts: "<file> keeps
  changing; nothing was saved".
- **The disk said no** (full, read-only, permissions), as now.

### Seeing changes

The TUI notices when a file it shows changes on disk, within about a
second, and redraws from it. This is about the view, not safety, and
lives in the TUI, not in the mechanism.

## Limits

- **Line numbers are read when a command runs.** `htt todo do 3` acts on
  whatever task is line 3 at that moment, as now. If another process
  changed the file since you last looked, that can be a different task.
  The TUI acts on the task it showed, by its text.
- **Other tools can still collide** in the instant between htt's check
  and its rename; htt narrows that to almost nothing but can't close it
  without their cooperation.
- **Network file systems** may not honour operating-system locks; on
  those, htt is as safe against itself as against other tools.

## Out of scope

- Merging two changes to the same task line (the transaction re-runs
  the operation; it doesn't merge text).
- A background service, a database, or a sync engine.
