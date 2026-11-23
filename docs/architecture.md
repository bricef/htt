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
  gui/                     htt gui: the local web server, its JSON API, and the page it embeds (web/)
  domain/                  Context, Task, Timelog + Repository / TimelogRepository interfaces
  storage/                 MemoryRepository / FileRepository (and timelog impls) — satisfy the domain interfaces; transaction.go is every write (safe writes)
  update/                  the once-a-day check for a newer GitHub release
  dates/                   date phrases ("next Friday") for --due and the TUI; --at's times
  editor/                  the user's editor and the command that opens a file in it, as git picks and runs one
  parseutils/              parsing helpers for the todo.txt format
  utils/                   small odds and ends (StringToFilename, EditFilePath, Fatal, …)
  vars/                    viper config keys + defaults
test/
  e2e/                     black-box tests that drive the compiled binary (and the GUI in headless Chrome)
  tui/                     TUI tests that drive the model through Update/View
  architecture/            asserts the import rules below
docs/
  architecture.md          you are here
  design/                  designs that stand as references: what a feature is and how it behaves, without how it's built
  plans/active/            in-flight design docs
  plans/complete/          historical record of merged design docs
  archive/                 older notes preserved for context
```

## Layering and the architecture test

The dependency graph runs one way:

```
cli ─────────────► tui, gui
 │                  │
 ▼                  ▼
storage ─────────► domain
```

`parseutils`, `dates`, `vars`, `editor` and `utils` are leaf helpers:
they import nothing else from `internal/` (but `utils` uses `editor`),
and the layers below may use them, with the exceptions noted.

- `domain` imports only `parseutils` and `vars`; never `storage`,
  `cli`, `tui` or `utils`.
- `storage` imports `domain` (plus `utils` and `vars`); never `cli` or
  `tui`.
- `tui` imports `domain` (it may also use `storage`); never `cli`,
  never `utils` (whose `Fatal` helpers exit the process, and whose
  editor helper runs the editor on the terminal the TUI owns), and
  never the standard `log` package (see [TUI](#tui)).
- `editor` imports nothing else from `internal/`. `Choose` picks the
  editor from the environment (`$VISUAL`, then `$EDITOR`), and
  `Editor.Command` builds the `*exec.Cmd` that opens a file in it;
  running it is the caller's: `utils.EditFilePath` runs it on the
  CLI's terminal, and the TUI's edit action hands it to Bubble Tea.
- `gui` imports `domain` only: never `cli`, `tui`, `storage`, `vars`,
  `utils` or `update`. The CLI hands it what it reads (see [GUI](#gui)).
- `cli` sits at the top and may import anything below (including `tui`
  and `gui`, to register the `interactive` and `gui` subcommands).
- `update` imports nothing else from `internal/`: it knows GitHub's
  release endpoint and its cache file, nothing about tasks or config.
  `cli` decides when it may run and passes the TUI a notice source.

`test/architecture/architecture_test.go` enforces the `domain`,
`storage`, `tui`, `gui`, `dates`, `editor` and `update` rules by scanning
imports in each package. If a future change breaks one, the test fails
and forces the discussion before the dependency lands.

## Domain types

### `Context`

A named bundle of `Task`s. The repo is injected at construction (the
`repo` field is private) and is the seam through which persistent
methods save changes back. Pure methods (`Search`, `Sort`,
`GetTaskById`, …) never touch the repo, so tests for those can
construct `Context`s via struct literals.

Persistent methods that go through the repo:

- `AddTask(*Task) error`
- `Delete(strID string) ([]*Task, error)` (the task and its subtree)
- `Replace(strID string, replacement *Task) (snapshot *Task, error)`
- `Move(strID, toName string) ([]*Task, error)` (the task and its subtree)
- `Complete(strID string, subtasks Subtasks) ([]*Task, error)`
- `SetPriority(strID, priority string) (snapshot, mutated *Task, error)`
- `IncreasePriority(strID string) (snapshot, mutated *Task, error)`
- `DecreasePriority(strID string) (snapshot, mutated *Task, error)`
- `AddSubtask(parentID string, task *Task) error`
- `Nest(strID, parentID string) (task, parent *Task, error)`
- `Unnest(strID string) (*Task, error)`
- `NestUnderPrevious(strID string) (*Task, error)`, `UnnestOneLevel(strID string) (*Task, error)` (the TUI's Tab and Shift-Tab)

`Move` and `Complete` save destination-first so a partial-save failure
leaves the task in both places (recoverable) rather than neither
(silent data loss). Every one of them saves through `Context.save`,
which arranges the tasks first (see [Subtasks](#subtasks)).

### `Task`

A single todo.txt entry. Constructors and presentation methods
(`NewTask`, `Entry`, `RawString`, `ConsoleString`, `Annotate`,
`RemoveAnnotation`) are exported. The mutator helpers (`setPriority`,
`increasePriority`, `decreasePriority`, `markCompleted`) are
unexported — the `Context` orchestrators are the only sanctioned
mutation path, since reaching for `Task` directly skips the persist
step.

`NewTask` parses with the grammar in `parser.go`, which reads a line
as words separated by whitespace (space, tab, CR, LF). Each token is a
whole word, and every word is one of them: the preamble (`x`,
priority, dates), a tag (`+`, `@`, `#`), a `key:value` annotation or a
plain word, non-ASCII text and a lone sigil included. That matters
because rewriting a task (a priority change, completion) rebuilds its
line from the parsed parts: a word the parser skipped or split would
be lost from the file. A parse that leaves text unread is an
`ErrMalformedTask`, never a partial task.

`rebuild` writes the preamble from the task's fields and then the rest
of the line word for word, from `body`: the words after the preamble
in line order, annotations marked with their key. Before writing,
`reconcileAnnotations` applies `Annotations` to it without moving
anything: a key's last occurrence (the one the map shows) takes the
current value where it stands, earlier repeats stay as written, a
removed key loses every occurrence, and new keys go at the end in key
order. So a rewrite changes only what was asked; words are rejoined
with single spaces.

One rewrite can still change what a task means: with less in front of
it (a priority or a leading annotation removed), the body's first word
can be read as preamble (`(C) x foo` lowered would be `x foo`, done).
todo.txt has no escape, so after composing, `rebuild` re-parses the
line (`bodyReadsBack`) and, if the body didn't survive, fills the empty
date slots with today, completed-on before created-on, until it does.
The parser stays the only judge of what the preamble takes; a regexp
of the words it can take (`preambleShaped`) only skips the re-parse
for lines that can't go wrong. The TUI's priority actions report an
added date on the status line (`datedNote`).

`isAnnotation` keeps times (`10:30`: an all-digit key) and links
(`https://…`, `mailto:…`) as plain words: they're part of what the
task says, not data about it. `Annotate` refuses the same pairs, so
htt never writes an annotation it wouldn't read back.

`Tokens` (`tokens.go`) returns a line's words with what the parser
read each as (plain word, completion mark, priority, date, each kind
of tag, annotation), for displays that colour them. It walks the parse
tree rather than re-implementing the grammar, and a property test
holds it to `NewTask`'s fields.

`ParseFilter` turns a filter expression (`overdue`, `due-soon`,
`no-due`, `has-tag:NAME`, `priority:X`) into a `Filter`, a predicate
over tasks, built on `Task.Due` and `Task.HasTag`. `htt todo show
--filter` and the TUI's search share them, so the two accept the same
words.

Every line is a valid task, whatever its dates: a date htt can't read
is text, and a `due:` value that isn't a date leaves the task without
one. `Task.DateWarnings` (`date_warnings.go`) says where that happened,
as messages naming the word: a date-shaped word that is impossible
(`2026-13-45`) or not zero-padded (`2026-5-3`), and the due: value
`Task.Due` reads when it isn't a `YYYY-MM-DD` date. It walks the words
`NewTask` already parsed. The CLI and the TUI both show these (see
[CLI wiring](#cli-wiring) and [TUI](#tui)); neither refuses the line.
`Task.Line` is the task's index in its context; `Task.FileLine`, set
by `FileRepository` and `FileTimelogRepository` as they read or save a
task or entry, is its line in the file (they differ once the file has
a blank line), for messages that point into the file.

### Subtasks

`docs/design/subtasks.md` is what subtasks are; this is how they're
built. A task with subtasks carries `id:N`, each subtask `p:N`, as
topydo writes them. `Task.ID` is a task's first `id:` and
`Task.ParentIDs` its `p:` values; ids are text, matched exactly, as
topydo matches them.

`NewTree(tasks, doneIDs)` (`tree.go`) reads a context's tasks as a
tree of `Node`s: a task's parent is the one task holding the id its
first `p:` names. A parent's subtasks are in priority order (`(A)` to
`(Z)`, then none; stable), top-level tasks in the order they came.
The broken links the design lists are `LinkProblem`s (a task index and
a message), each affected task at the top level: a `p:` naming no task
(nor a done id), every task of a loop, an id on several tasks (the
problem is on each holder; the tasks naming it go to the top level),
and several `p:` (the task stays under its first). A `p:` naming one of
`doneIDs` is a done parent: the task is at the top level, marked
(`Node.DoneParent`), with no problem. `Context.Tree` supplies the ids
of the context's tasks in the done file (those whose `context:` names
it, as `Complete` writes them), reading `done.txt` only when a link
doesn't resolve in the file (`Tree.Unresolved`), once until the
context is next saved, and, from a repository that is a `LineSource`
(both of storage's: `ContextLines` hands over a context's lines
unparsed), parsing only the lines with an `id:` and the context's
`context:` in them: parsing a large done file whole took seconds. `NextID` is
topydo's `find_next_id`: the smallest whole number no `id:` or `p:` in
the file uses.

The done file and the archive collect tasks from every context, so
their ids aren't one file's: `Context.IsTree` is false for them, and
they stay flat, in the order written.

**Order.** `Context.Arrange` puts the tasks in tree order (a
depth-first walk, `Tree.Order`) and numbers them in it (`Task.Line`);
it only moves pointers, never rewrites a line. It runs on every write
(`Context.save`, which every persistent method saves through) and on
every load (`FileRepository.Context` and `MemoryRepository.Context`
call it), so a task's number is its place in the tree even in a file
another tool left out of order, and the tree reads top to bottom in
number order. `FileLine` stays the line in the file, for warnings. A
priority change still sorts the context by priority (`Sort`, now
stable) before the write, as in 1.0, so top-level tasks sort as they
did; arranging then puts each subtask among its siblings.

**Operations** (`subtasks.go`) each refuse, changing nothing, with an
error the CLI and TUI word: `ErrNotATree` (done file, archive),
`ErrLoop` (checked by following the tags, so two tasks already in a
loop can't close it again), `ErrSharedID` (a parent whose id another
task holds: a `p:` naming it couldn't resolve), `ErrTopLevel`,
`ErrNothingAbove`, `ErrOpenSubtasks`. A parent gets its id on its
first subtask (`idFor`). Where a task lands is the list's order, then
arranging: `AddSubtask` appends (so the new task lands after its
parent's last descendant, or at its priority's place); `Nest` moves the
subtree to just after the new parent's subtree (last among its
priority). The others needn't move anything, the list being in tree
order: under `NestUnderPrevious` the task already follows the subtree
of the task above it, and a task that `Unnest` or `UnnestOneLevel`
takes out of a subtree already sits right after it, where arranging,
which keeps the order of the top level and of siblings of a priority,
leaves it.
`Move` takes the subtree (refused to the same context, names compared
as files are named, `ContextFileName`), and into a context file
`fitInto` removes the moved task's `p:` and renumbers the subtree's
ids the target already uses (as an `id:` or a `p:`), with their `p:`
links; a done task going back to the context it was completed in
keeps the id the `p:` there name it by, and its `p:` when its parent
is there. Taking a task's `p:` away (`takeParent`: `Unnest`,
`UnnestOneLevel` to the top, `fitInto`) refuses a task with no other
text (`ErrOnlyLink`). `Delete`
archives the subtree. `Complete` takes a `Subtasks` choice:
`SubtasksAsk` (the zero value) refuses a task with open tasks below it,
`SubtasksLeave` completes it alone, `SubtasksComplete` with every open
task below it (`OpenSubtasks`). `SubtasksQuestion` is the question the
CLI and the TUI ask, and `SubtaskLine` the line `AddSubtask` would
save, for the TUI's preview.

### `Timelog`

A day's worth of activity entries. Each entry is a `Task` annotated
with a `ts:<RFC3339>` timestamp, in UTC, by `Timelog.Append` or
`AppendAt`. The persistent methods are `Append`, which stamps now and
adds the entry at the end, and `AppendAt(task, at, now)`, which stamps
`at` and puts the entry right after the last entry at or before it (or
first), so a day in time order stays in order and the newest of
entries at the same time is the one that runs. `AppendAt` refuses,
changing nothing, a time later than `now` (`ErrFutureEntry`) and one
whose date in `now`'s zone isn't the log's. The CLI's `--at` (on `log
start`, `add`, `end` and `workon`) reads the time with
`dates.ParseAt`, loads the day it names and calls `AppendAt`. The pure
methods are `IsEmpty` and `Timesheet` (`timesheet.go`), which reads
the day as `Interval`s: each entry runs until the next one; an `@end`
(`IsDayEnd`: the entry is just `@end`) makes none, so the gap after it
belongs to nothing; and the day's last entry runs to local midnight
(`DayUnended`, which the report flags with the `htt log end --at`
that ends it) unless it is an `@end` (`DayEnded`), or, today, until
now (`DayInProgress`). Intervals never leave their day. `Timesheet` takes `now`, and `now`'s zone is the
local one, so tests pin both. `htt report` reads the sheets with
`ReadDays` and adds them up with the period functions (below); `htt
log status` reads `Status`,
`EndedAt` and `Current`, whose `Category` (its name, and whether it
is work) and `Uncategorised` it shows. `log status` and `htt status`
(not `log active`) then find the most recent day before today with
entries, up to 14 days back, with `LastLogBefore(repo, now, days)`
(`timelog_repository.go`), which walks back through `Day` a calendar
day at a time in `now`'s zone (each day at noon, since a midnight can
be missing), and add a line with the `htt log end --at` that ends it
when that day's sheet is `DayUnended`. Only that day: older ones are
the report's.

Each interval has a `Category` from a `Categoriser`'s `Match`
(`timelog_category.go`): the first category with one of the entry's
tags, matched exactly; or, when none has one, the default category
(`working`, or the one `WithDefault` names, whose `Work` applies), and
the interval is `Uncategorised`. `ByCategory` is the time categories'
tags matched, the default's own tags included, and `Uncategorised` the
rest, so the report can show unmatched time on its own line instead of
under the default's name; `Worked` counts it when the default counts
as work. `Work: false` keeps a category, such as breaks, out of worked
time. The domain holds no config: `cli` reads `timelog.categories`
and `timelog.default_category` (`timelog_config.go`) and builds the
categoriser, as it builds the TUI's theme. `NewCategoriser` and
`WithDefault` check the categories, and `cli` names the config key in
the error. A bad config stops `htt report` before it prints; `htt log
status`, `log active` and `htt status` warn on stderr and show the
entry without categories (`Timesheet` with a nil categoriser).

Days add up into periods in `timesheet_period.go`, pure functions over
a day's `Timesheet` each, in date order; `htt gui` draws from them. A
sheet's `Date` is its day at noon, since its `Start` (time.Date's
midnight) can be on the day before where the clocks go forward at
midnight. `Totals` is time added up: `Worked`, `Breaks` (the rest),
`ByCategory`, `Uncategorised` and `ToMidnight` (the last entry of each
day without `@end`, counted to 24:00); `Timesheet.Totals`, `Plus`,
`Sum`, and `Scale(num, den)` for averages and rates per week. A
`Period` is a run of calendar days, each a date at noon, so they're
counted by date and not 24 hours apart: `WeekOf` (Monday to Sunday)
and `MonthOf`. Over consecutive days, `Weeks` cuts Monday-to-Sunday
weeks at the days' edges, `Halves` splits a month at the 15th,
`Rolling(days, 7)` is each day's total of the 7 days up to it (pass the
6 days before a period for its first days), `WeekdayAverages` averages
each weekday, and `Summarise` gives the averages per week and per
working day and the days without `@end`. Their property tests check
them against direct sums of the intervals over generated months.

A week's and a month's figures are put together once, in
`timesheet_figures.go`, for both `htt gui` and `htt report --week`
and `--month`, so the browser and the terminal show the same numbers.
`CalendarDate` reads a date or a month as a day at noon in the local
zone (where the clocks go forward at midnight, that midnight doesn't
exist). `ReadDays(repo, categories, dates, now)` reads each date's log
through `TimelogRepository.Day` as a timesheet at `now`; a day after
today it doesn't read, so a log written by hand for tomorrow counts
for nothing. It reads through the interface, as `LastLogBefore` does,
so the domain needs no storage. `SoFar` is the days up to today.
`Week` and `Month` read a period and add it up over its days so far,
today included (`WeekFigures`: the days and the summary;
`MonthFigures`: the summary, all the month's weeks, the weekday
averages, the rolling weeks, reaching 6 days into the month before,
the halves, and the category table). The table
(`CategoryTable`: a row per category with tags, `Tagged`, then
uncategorised) holds each category's total, its share of worked time
for one that's work, its time per week in each half
(`Stretch.PerWeek`) and the change; `Summary.UncategorisedShare` is
the summary's share. The GUI turns these into JSON and the report
into text; neither computes a figure of its own.

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
    Transact(change func(tx Repository) error) error
}

type TimelogRepository interface {
    Today() (*Timelog, error)
    Day(date time.Time) (*Timelog, error)
    Save(l *Timelog) error
    CurrentLogPath() string
    LogPath(date time.Time) string
    Transact(change func(tx TimelogRepository) error) error
}
```

`Transact` is how every change is made: see [Safe writes](#safe-writes).

Two implementations of each in `storage`:

- `MemoryRepository` / `MemoryTimelogRepository` — in-memory, for
  tests. They deep-copy tasks and entries on load and save so
  mutations through a returned pointer can't leak back into stored
  state. This matches the file impl's serialize-then-reparse
  semantics. Their transactions hold saves until the change returns
  and run it again when the store changed meanwhile; their
  `Underneath` hook, run between a change and its check, lets a
  test change the store there. No lock.
- `FileRepository` / `FileTimelogRepository` — todo.txt-format files
  on disk. Both task repositories arrange a context as they load it
  (`Context.Arrange`), so its tasks come in tree order, and both are
  `domain.LineSource`s (`ContextLines`: a context's lines, unparsed). The on-disk layout is byte-for-byte compatible with the
  pre-refactor format. Context names are sanitized
  (`utils.StringToFilename`) at the path boundary so a name like
  `../escape` can't write outside `dataDir`. Every write goes through
  `transaction.go` (below); the previous version of a context file or
  a log stays as `.bak`, one slot of crash recovery.

`FileRepository` takes two directories: `dataDir` for per-context
files and `pointerDir` for the `current-context` pointer. Default
config maps both to the same directory, but users who set
`tracker_path` independently of `data_path` keep their split.

## Safe writes

Several htt processes (the CLI, the REPL, a TUI, a script) and other
programs (an editor, topydo, a sync tool) write the same files. The
design is [docs/design/safe-writes.md](design/safe-writes.md); this is
where it lives.

**The mechanism is one file, `internal/storage/transaction.go`**, and
nothing else in htt locks a file, writes a temporary one or renames
one over another (an architecture test pins it). `transact(dirs,
hooks, change)` runs a change as a transaction:

1. **Lock** `.htt.lock` in each directory the transaction can write
   (a `FileRepository`'s data and pointer directories, one lock when
   they're the same; a `FileTimelogRepository`'s log directory), in
   sorted order, with `github.com/gofrs/flock` (`flock` on Unix,
   `LockFileEx` on Windows). An operating-system lock goes with its
   process. It waits `lockWait` (5 s) at most, then gives up with
   `domain.ErrBusy`.
2. **Read** through the `transaction`: each file read once, fresh, its
   bytes kept; a file the change has written reads as written.
3. **Change:** writes are held, in order.
4. **Write** (`stage`): each file the change wrote, and its previous
   version for `<file>.bak` (not for the pointer), goes to a temporary
   file beside it (`.<name>.<random>.tmp`, created exclusively, the
   files' mode and the umask as always), written, synced and closed.
   The `.bak` is the bytes read, which the check covers; a file written
   unread is read now and checked too. A file that is a symlink (to a
   file a sync tool keeps elsewhere, say) is followed (`realPath`): the
   temporary file goes beside the file it points to, which is replaced,
   and the link stays a link. Temporary files a crashed htt left in a
   directory written, an hour old or more, are removed (`sweep`).
5. **Check** that every file read still has the bytes read (or still
   doesn't exist). If not, another program wrote it: remove the
   temporary files and run the change again from 2, up to 5 runs, then
   `domain.ErrKeepsChanging` ("<file> keeps changing; nothing was
   saved"), writing nothing.
6. **Replace:** rename each temporary file over its file (each `.bak`
   just before its file): the files that keep every line they had
   (gaining tasks) before those that lose one (`gainingFirst`), each
   group in the order the change wrote them, so a composed change that
   saved the losing file first still can't leave a task in neither. Only
   renames follow the check, so the moment in which another program's
   write can go unseen is the renames' alone, not the writes' and
   syncs' too. On Windows a rename over a file another process has
   open fails (Go opens files without `FILE_SHARE_DELETE`), so it's
   tried again for up to two seconds.
7. **Unlock.**

Reads outside a transaction (showing a list) take no lock: a file is
always whole.

**The repositories** only parse and serialise. `FileRepository.Transact`
hands the change a `fileTx`, a `domain.Repository` (and `LineSource`,
`SameFiler`) whose `Context` parses what the transaction reads and
whose `Save` and `SetCurrent` hold what they're given; `Transact` on it
runs a change within the same transaction. `FileTimelogRepository`
does the same with `fileTimelogTx`. `Save` and `SetCurrent` outside a
transaction are transactions of one write. `hooks` are seams for
storage's own tests: `writing` before each temporary file is written
(to fail one), `underneath` between the written files and the check,
`replacing` before each file is replaced (to fail one).

**The domain** declares `Transact` on both repository interfaces, and
its one save point runs through it: every persistent `Context` method
calls `transact`, which runs the method's body as a transaction of the
context's repository on the context as the store holds it then. The
context's tasks become the file's, its own task objects for the lines
it still holds, so a task the caller holds sees the change; its
repository is the transaction's while the body runs, so the done file,
the archive or a move's target the body loads and saves are part of
it. The lock is held while the body runs, so it parses no more than it
must: the context's lines come unparsed (`LineSource`) and are compared
with those it holds, and only the lines it doesn't hold are parsed
(`reread`), so a command's context isn't parsed a second time; and the
done file and the archive are appended to (`domain.Appender`, which a
`fileTx` is: the lines go after the bytes read), never parsed, however
many tasks they hold. The body saves the file gaining
tasks before the one losing them, and the storage writes the gaining
file first whatever the order of the saves, so a failure part-way
leaves a task in both, never in neither.
A body may run again, so it decides from the context alone; a task
number is read then (the design's Limits), and `AddSubtask` puts its
task back as it was given before running again. `Context.Transact`
runs several steps as one transaction, for the TUI. A command that
waits for the user follows its task by the line it read
(`follow.go`): `FindTask` finds it where it was or, moved, as the one
task with that line; `ReplaceFound` (the CLI's `todo edit`) and
`CompleteFound` (the answer to `todo do`'s question, in the CLI and the
TUI, which also checks the open subtasks are the ones asked about)
refuse with `ErrTaskChanged` or `ErrSubtasksChanged` otherwise. `Timelog.Append`
and `AppendAt` run the same way. The CLI, the REPL and the GUI call
the operations as before and see only `ErrBusy`, `ErrKeepsChanging`,
or the disk's own errors.

## CLI wiring

`internal/cli/app.go` is the injection seam:

```go
func repo() domain.Repository
func baseRepo() domain.Repository
func SetRepository(r domain.Repository)

func timelogRepo() domain.TimelogRepository
func baseTimelogRepo() domain.TimelogRepository
func SetTimelogRepository(r domain.TimelogRepository)
```

Each is lazily initialized from viper config. Tests inject memory
impls via `SetRepository` / `SetTimelogRepository`. Cobra commands
call `repo()` / `timelogRepo()` at the top of each `RunE`.

`Execute` (`root.go`) runs a command: `main` runs the command line
through it, and the REPL each line. It reads a bare `-` where todo's
command goes as `down` first (`minusAsDown`), so `htt todo - 3` lowers
task 3's priority: cobra takes any word starting with `-` for a flag
before it looks up a command, so an alias can't do it, as `+` does
for `up`.

`repo()` wraps the configured repository (`baseRepo()`) in
`warningRepository` (`date_warnings.go`), which prints
`warning: <file>:<line>: <message>` on stderr for each
`Task.DateWarnings` of a context a command loads, once per run (by
file, task text and message, so a REPL session that moves a line
doesn't repeat it). Only stderr: stdout and the exit code are the
command's. Contexts the domain loads itself, to append to (done.txt
on completion, the archive on deletion, a move's target), go through
the wrapped repository and don't warn, and the archive never warns.
`htt interactive` hands the TUI `baseRepo()`: the TUI owns the
terminal and shows the warnings on its status line.

`timelogRepo()` does the same for time logs: it wraps the configured
repository in `warningTimelogRepository`, which warns about the
entries of each day's log a command loads (`Today`, `Day`), in the
same form and with the same once-per-run record, naming the file
`LogPath` gives. An entry's `ts:` timestamp isn't a date
`Task.DateWarnings` reads, so it never warns. `Timelog.Append` and
`AppendAt` save through the wrapped repository. Status's look at a
past day (`printUnendedDay`) goes through `baseTimelogRepo()`, the
repository unwrapped: it reads that day only for its `@end` and shows
none of its entries, so their dates are the report's to warn about.

`htt report` (`report.go`) prints the tasks completed, added and
deleted in its period, then the time logged. `--since` reads its days
with `ReadDays` and adds them up with `Sum`; `--week` and `--month`
(`report_period.go`) ask the domain for `Week` or `Month` at the
clock to the second, as `htt gui` does, read before anything is
printed, and lay the figures out: `--week` its days as `--since`
prints them (`printDays`) and its summary, `--month` its summary and
tables. A period's figure is rounded to the second, as the GUI's API
gives it, before the report's hours and minutes drop the seconds, so
both show the same minute. Which of `--since`, `--week` and `--month`
was given is read from the flags' own values (`--since`'s through
`givenString`), never pflag's `Changed`, which stays set between the
REPL's commands; `resetReplFlags` resets them.

The same wrapper warns about subtasks' broken links
(`domain.LinkProblem`, from `Context.Tree`), in the same form and with
the same record, each task's after its dates. `printContext`
(`show.go`) prints a context for `htt todo`, `todo show`, `todo
status`, `htt status` and the REPL: the tree, two spaces a level after
the number, `(parent done)` after a done parent; `--flat`, sorted and
filtered views and `search` print the plain list.

`todo do` (`complete.go`) asks before completing a task with open
subtasks: `onTerminal` (stdin and stdout both terminals, the REPL's
check too) or it fails naming `--leave-subtasks` and
`--with-subtasks`. The question runs through `runOnTerminal`, so in
the REPL it takes the terminal from readline as the TUI does, and
`readAnswer` reads one line a byte at a time, taking nothing after
it; only `x` and `A`, exactly, answer.

Commands that write a task the user typed (`todo add`, `add-to`,
`replace`, `edit`, and `log add` for a time-log entry) warn about it
once it's saved, through `warnTypedTask` / `warnTypedEntry`, rather
than leaving it to the next load: saving set its `FileLine`, so the
warning names the line it went to (for `log add --at`, in the day's
file it went to, wherever in it). These share the once-per-run
record, so the REPL's reload after the command doesn't repeat them.

## TUI

`internal/tui` is a Bubble Tea program. `tui.Model(repo, ctx, opts)`
builds the model from `Options` (a logger and a theme; the zero value
means a discard logger and the default theme). `htt interactive`
(`internal/cli/interact.go`) builds both, the logger from
`--log-file` / `--log-level` and the theme from the config, then runs
the program through `runOnTerminal`, which just runs it.

In the REPL (`internal/cli/repl.go`), `runOnTerminal` is the REPL's
`replTerminal.handOver`, so `/interactive` runs the TUI on the REPL's
terminal. readline reads the terminal on goroutines of its own, so
`handOver` closes the REPL's readline instance, which stops them,
runs the program, and opens a new instance on the same history file,
even when the program fails. The REPL then draws its view as after any command; the TUI
saves a context switch (`SetCurrent`) and every change, so the view
is the context the TUI left. The REPL saves each line to the history
itself (`replReader.Readline`) rather than leave it to readline,
which saves after returning the line, on its own goroutine, and could
lose the `/interactive` to the close. A REPL whose stdin or stdout
isn't a terminal doesn't close anything: the program gets an error
saying the TUI needs a terminal. The REPL refuses `repl` and `gui`,
whatever alias names them (`refusedInRepl` asks cobra which command
a line runs).

- **Actions.** Each key binding maps to an `Action`: a description
  plus a `func(model) (tea.Model, tea.Cmd, error)`. The action decides
  what happens next (carry on, `tea.Quit`, `tea.ClearScreen`, run a
  program); `Action.Act` is the one place that logs it: every action
  at debug, an error wrapping `errUnavailable` (the action can't run
  right now) at warn, any other error at error. Mutating actions run
  through `onShown`: one transaction (`domain.Context.Transact`) that
  finds the selected task in the list's file as it is, by the line the
  TUI showed (`shownAt`, the domain's `FindTask`), and acts on it
  (`c.X(...)`), so nothing older
  than what another htt wrote is saved, and a run again after another
  program's write finds the task again. A task changed or removed
  elsewhere is left alone (`errGone`), and the status line says so
  (Tab also checks, inside the transaction, the task above it, and the
  answer to `x`'s question the subtasks it counted). A save that fails
  (another htt busy with the files, a file that keeps changing, a full
  disk, a read-only directory) goes on the status line (`failed`) and
  the TUI stays open, as for a tab switch that fails: nothing was
  saved, and the next key works on the file as it is. Then `refresh`, which
  reloads the list shown by its name, as the watch does (another htt
  switching the current context doesn't switch the TUI's), and
  `showTasks`. Context-switching actions (`switchTo`) load the tab's
  context by its name, then call `m.repo.SetCurrent`.
- **Watching** (`watch.go`). `Init` starts a tick (`watchMsg`) each
  second for a list with a file (`ContextPath`); each tick stats it
  (`look`) and, when it's another file (an atomic replacement), or
  its size or modification time changed, since the last look
  (`model.seen`), reloads the list (`reloadShown`) and shows it, the
  selection on the same task by its text. What holds a task by its
  number across the reload follows it by its text too: the task input
  open for an edit or a subtask (`followInput`: its prompt, preview and
  check name the task's number now; a task gone leaves the input open
  with what was typed, saying it can't be saved, and Enter closes it)
  and the question about open subtasks (`followQuestion`). A search
  is applied again to the list reloaded. A reload that finds the
  same lines changes nothing. After an action, `refresh` looks at the
  file before reading it, so the TUI's own write isn't taken for a
  change on the next tick.
- **The task list** is a `bubbles/list` model (`tasklist.go`). Each
  item is a `taskItem`: a task and its index in the context. Actions
  take that index from `selectedTask()`, never the row on screen,
  because a search changes which row a task is on. `showTasks` reloads
  the items, re-applies an active search synchronously and can select
  a task by index (how a priority change keeps the cursor on its
  task). The list owns moving, paging and the search box (`listOwns`);
  its default keys that clash with htt's (`h`, `l`, `f`, `d`, `q`,
  `?`) are taken back in `listKeys`. Every message that isn't a key
  htt handles goes to the list, including the asynchronous search
  results. Search is `searchFilter`: words that `domain.ParseFilter`
  accepts are filters, the rest fuzzy text, all ANDed, file order
  kept.
- **The tree.** `showTasks` loads the items from the context's tree
  (`Context.Tree`), in its order, leaving out the subtrees of folded
  tasks (`model.folded`, by context and id, for the session); each
  `taskItem` carries its depth, how many tasks its fold hides and
  whether its parent is done. While a search is typed or applied every
  task is an item, folded or not, and rows are drawn flat;
  `updateList` reloads the items when a search starts or ends (only
  when something is folded). Space folds (`ToggleFold`), `s` opens
  the input for a subtask (`inputSubtask`, previewed with
  `SubtaskLine`), Tab and Shift-Tab call `NestUnderPrevious` and
  `UnnestOneLevel`, and the cursor follows the task moved
  (`arranged`, unfolding what's above it). `x` on a task with open
  subtasks sets `model.asking` and puts `SubtasksQuestion` on the
  status line; `update` hands the next key to `answer` before
  anything else. Link problems join date warnings on the status line
  (`noteDateWarnings`).
- **Task rows.** `taskDelegate` draws each row through `styles.line`,
  which colours the line word by word from `domain.Tokens`: the same
  parse `NewTask` makes, walked for each word's kind and byte offsets,
  so a colour always matches what htt understood. `id:` and `p:` are
  drawn muted (`styles.muted`), as are a row's notes, `(N folded)` and
  `(parent done)`, after the line; a subtask is indented two spaces a
  level after the number. The search's match
  positions (`list.MatchesForItem`) are byte offsets too, from
  `sahilm/fuzzy`, and are underlined. The selected row keeps one
  style, and rows are cut to the list's width with `ansi.Truncate`.
  Rows are tokenised as they're drawn (about 50µs a line), so there's
  no cache to keep in step with the tasks.
- **The task input** (`input.go`) serves adding (`n`) and editing
  (`e`); `model.input` says which, and an edit remembers the task's
  index and line. `readDraft` turns what's typed into the line a save
  writes: `due:` phrases resolved through `dates.ParsePhrase`, and for
  a new task `Task.StampCreated`, the same stamp `Context.AddTask`
  applies, so the preview can't differ from the file. Its problems
  block the save and keep the input open; its warnings
  (`wordWarnings`, from `domain.Tokens`, with the dates judged by
  `Task.DateWarnings` as on load) don't. `AddTask` and
  `ReplaceTask` save; `ReplaceTask` refuses if the task at the index
  is no longer the one being edited. Tab completion uses the text
  input's own suggestions, which complete the whole value by prefix:
  `tagSuggestions` offers the value with its last word replaced by
  each tag `knownTags` gathered from every context but the archive
  when the input opened.
- **The terminal.** The TUI owns the terminal while it runs, so
  nothing in `internal/tui` writes to stdout or stderr: diagnostics go
  to the model's `*slog.Logger`, and anything the user should see goes
  on the status line (`model.status`, cleared on the next keypress).
  The architecture test keeps `log` and `utils` out of the package.
  Date warnings go there too: after every update, `noteDateWarnings`
  checks the list on show if it was (re)loaded since it last looked,
  and adds the first new `Task.DateWarnings` (by task number, with a
  count of the rest) to whatever the action said. It remembers what
  it has shown (by list, task text and message), so each line warns
  once a session. A test in `test/tui` checks nothing reaches stdout
  or stderr meanwhile.
- **Themes.** `Theme` holds one colour per role; the built-in palettes
  live in `theme.go`. `cli` reads the config's `tui.theme` and
  `tui.colors` and calls `tui.ResolveTheme`, so the tui package never
  reads config, and passes the result in `Options`. The model builds
  its `styles` from it once; `View` takes every colour from there.
- **Other programs.** The edit action gets the editor's command from
  `internal/editor`, as the CLI's edit commands do, and runs it through
  `tea.ExecProcess`, which releases the terminal while the editor runs
  and sends `editorFinishedMsg` back to `Update` when it exits.

## GUI

`htt gui` (`internal/cli/gui.go`) serves the GUI from the binary: a
local web page that draws the time log, and the JSON it draws from
(`internal/gui`). The CLI builds a `gui.Config` (the timelog
repository, without date warnings, as the page refreshes every
minute; the `Categoriser`, or the config's error; and the clock),
`gui.New` makes the server, and `ListenAndServe(stop, port, ready)`
listens on 127.0.0.1, calls `ready` with the address,
`http://127.0.0.1:<port>/` (which the CLI prints and opens), and serves
until a signal on `stop`, the CLI's `signal.Notify` channel for Ctrl-C
and SIGTERM. Then it stops taking requests, gives those under way a
second to finish (a second signal cuts it short), closes the
connections left, a browser's idle ones among them, and returns nil.
The REPL refuses `gui`.

`openBrowser` opens the address in the background, with htt's own
opener (`opener_unix.go`, `opener_windows.go`): `open` on macOS;
elsewhere the first of `xdg-open`, `x-www-browser`, `www-browser` and
`wslview` on `PATH`, then, on WSL without wslu, `explorer.exe`, which
hands an address to Windows' default browser (and exits 1 when it has,
so its status is ignored); ShellExecute on Windows. On Unix it runs in
a process group of its own: the terminal's Ctrl-C goes to htt's group,
and the opener may be the browser itself. A failure to open is a line
on stderr; an opener stopped by a signal isn't one.

`Server.Handler(port)` is the server's guard, in this order:

1. A `Host` that isn't `127.0.0.1:<port>` or `localhost:<port>` (or,
   on port 80, either without the port, as browsers send it) is a 403
   (DNS rebinding).
2. A request from anywhere but the GUI's own page is a 403
   (`foreign`): an `Origin` that isn't `http://` and one of those
   hosts, `null` included; or, by its `Sec-Fetch-Site`, a request
   another site's page makes (`same-site`, as a page on another port
   of localhost is, or `cross-site`), unless it's a link followed to
   open the page, a GET with `Sec-Fetch-Mode: navigate` and
   `Sec-Fetch-Dest: document`. `same-origin` (the page's own
   requests), `none` (an address typed, a bookmark, the opener) and no
   `Sec-Fetch-Site` at all (curl) pass.
3. A method other than GET or HEAD is a 405.

Every answer, the 403s included, carries a Content-Security-Policy
keeping the page to its own files, `nosniff`, `no-referrer` and `DENY`
framing; the API's are `no-store`. The page is at `/`; `/api/week`
and `/api/month` are matched exactly, anything else under `/api/` is a
404 naming them, and anything else is a file of `web/`, served only if
it's a regular file (a valid `fs` path, so `..`, `.` and empty
elements reach nothing).

There's no token or login, as in Go's own local read-only UIs (`go
tool pprof -http`, `go tool trace`). A page elsewhere can't read the
answers (same-origin policy: the server sends no CORS headers), nor
have them loaded at all, as an image, a script, a style or a frame
(Origin and Sec-Fetch-Site); DNS rebinding meets the Host check;
there's no write to forge; and no other computer reaches 127.0.0.1.
What's left open is other accounts on the same computer, which can
open the page while `htt gui` runs. `http.CrossOriginProtection`
isn't used: it leaves GET and HEAD alone, the only methods the server
answers. If the GUI ever writes, it's the guard for the writes.

The API (`view.go`, versioned there in `apiVersion`): `api/week?date=`
any day of the week, `api/month?month=YYYY-MM`, each today's by
default, read as days on the calendar (parsed in UTC, then noon in the
local zone: where the clocks go forward at midnight, local midnight
doesn't exist). Go computes, the page draws: the handlers ask the
domain for the week's or the month's figures (`domain.Week`,
`domain.Month`) at the server's clock (to the second), the figures
`htt report --week` and `--month` print, and answer a view: `now`, `today`,
the `categories` (in the config's order, those with tags, then
`uncategorised`; each with `work` and a colour index, its place among
the work categories or among the others), the `period` (its key and
those of the periods before, after and holding today), and the `days`:
each one's status (`empty`, `ended`, `inProgress`, `unended`, or
`future` for a day after today, whose log isn't read), worked time,
breaks, time to midnight, time per category (uncategorised under
`uncategorised`, a name no category can have) and intervals (the
entry without its `ts:`, its category, `uncategorised`, `work`, start
and end, `seconds`, and `open`: `now` or `midnight`). Durations are
seconds; times are RFC 3339 with the local offset, each with its
minutes since local midnight on the wall clock (`startMinute`, 1440
at the day's end), for drawing, so a day the clocks change draws by
its clock while durations stay true; where the wall clock runs
backwards, in the hour that repeats when the clocks go back, an
interval is drawn for its true length, to its `drawnEndMinute`. A
`summary` comes with both; the week adds `hours`, the stretch its
timeline opens on, and the month
`weeks`, `weekdays`, `rolling`, `halves` and the `table` of
categories. The figures (summary, averages, rolling totals, halves)
are over the days so far, today included. A bad date is a 400; a
config htt can't use, or a log it can't read, a 500 whose `error` the
page shows.

The page (`web/`: `index.html`, `style.css`, `app.js`) is plain HTML,
CSS and JavaScript with no build step, embedded with `go:embed`. It
routes on the hash (`#week/2026-11-16`, `#month/2026-10`), fetches the
view, and draws it with ECharts (SVG renderer); it refetches when it
is seen again and every minute while the period holds today. Colours
are CSS custom properties, light and dark; the charts read them, so
`app.js` draws again when the system's setting changes. The
third-party files are in `web/vendor`, with their licences and a
README giving each one's source, version and sha256: ECharts 6.1.0,
and IBM Plex Sans and Mono (woff2, Latin1). Nothing is fetched from
the network.

Tests: `internal/gui` drives the handler with an in-memory repository
and a fixed clock in London, and compares a week and a month with
golden files in `testdata` (`go test ./internal/gui -update` rewrites
them, to be read before they're committed). `test/e2e/gui_test.go`
runs `htt gui --no-open --port 0`, reads the address from stdout, and
stops it with an interrupt (not on Windows, where Go can't send one).
`test/e2e/gui_opener_test.go` puts a stand-in `xdg-open`/`open` on
`PATH` (not on Windows, where ShellExecute opens the browser) and
checks that it's given the address htt printed, and nothing else; that
one that fails is reported; and that the terminal's Ctrl-C, sent to
htt's process group, leaves it running.
`test/e2e/gui_browser_test.go` loads the page in headless Chrome
(chromedp) and fails on a JavaScript exception or a console error:
both views; the corners a review found (categories named after
`Object.prototype`'s members, a pinned entry and another week, an
average week above every bar, the hour that repeats). It finds Chrome
in `$HTT_TEST_CHROME` (which may be a script that sets up the
browser's environment and execs it), on `PATH`, or where macOS and
Windows install it, and is skipped without one.

## Update check

`internal/update` asks GitHub for the latest release
(`releasesURL`, fixed at build time) and compares its tag with the
binary's version, `CurrentVersion`: the `version` variable when a
build sets it with `-ldflags -X` (a source-archive build, which has no
git information), otherwise Go's build info. `htt --version` prints the
same. `Start` records the attempt in
the user cache directory before the request and runs the request in
the background; `Check.Newer` reports a newer release only on the run
that asked, and every failure is kept in `Check.Err` rather than
returned. The root command's `PersistentPreRun` starts at most one
check per process when `updateCheckAllowed` permits (config on,
someone at a terminal, not completion or help); `main.go` prints the
notice after `Execute`, and `htt interactive` hands the TUI
`Options.UpdateNotice` for its status line. What it may contact and
send is promised in the README's Online activity section, and the
tests pin it.

## Adding a feature

Most features touch one or two layers. The usual path:

1. **Domain change.** If the feature needs a new operation, add it as
   a method on `Context` / `Timelog` (or as a new domain type). Pure
   methods are testable with struct literals; persistent methods are
   testable through `MemoryRepository`. A persistent method runs its
   body through `transact` and saves only with `save` (see [Safe
   writes](#safe-writes)): the body may run again on fresher content,
   so it decides from the context alone, and saves the file gaining
   tasks first.
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
   (`withMemoryRepo` / `withMemoryTimelogRepo`); output belongs in the
   e2e suite, since in-process tests can't see stdout. For TUI
   features, `internal/tui/actions_test.go` (actions and their
   logging, against a memory or file repo) and `test/tui/` (drives the
   model through `Update` / `View`).

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
- `2026-11-18-timelog-categories.md` — how a day's entries become
  intervals, what `@end` and the 24:00 rule mean, and why categories
  are a list matched by exact tags.
- `2026-11-18-timelog-at.md` — what `--at` takes, where an entry
  logged at a time goes, and what a time the clocks skip becomes.
- `2026-11-19-gui-time-log.md` — why the GUI is a local web page
  served by htt (no cgo), how it stays private, and what its JSON
  contract promises.
- `2026-11-20-report-periods.md` — `htt report --week` and `--month`,
  and why the GUI and the report take their figures from one place.
- `2026-11-02-tui-log-file.md` — why the TUI has its own logger and
  how actions report errors.
- `2026-11-23-subtasks.md` — how subtasks were built on
  `docs/design/subtasks.md`: where the tree lives, how the file order
  hooks into reads and writes, the prompt, the TUI's keys.
- `2026-11-23-safe-writes.md` — how safe writes were built on
  `docs/design/safe-writes.md`: the transaction's shape in the domain,
  the lock files and the wait, the check, Windows' rename, and what
  each concurrency test caught.
- `2026-11-02-tui-editor-exec.md` — how the editor gets the terminal,
  and why editor failures never close the TUI.
