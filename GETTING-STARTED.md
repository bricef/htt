# Getting started with htt

A fifteen-minute tour. By the end you'll have added a few tasks,
prioritised them, completed one, organised the rest into contexts,
logged some work against them, and seen the rollup status. The later
sections double as the reference for every command.

If you just want the install + overview, [`README.md`](README.md) has
the quick version. This doc walks through a realistic workflow.

## Install

With a working Go toolchain:

```
$ go install github.com/bricef/htt/cmd/htt@latest
$ htt --help
```

The first run creates `~/.config/htt/config.yaml` with defaults. Your
task files live under `~/.htt/data/`.

```
$ htt
Could not find a configuration file.
Creating a default file at /home/you/.config/htt/config.yaml.
htt is a command line todo list manager and time tracker
…
```

Two global odds and ends: `--no-color` turns off coloured output for
any command, and `htt completion --help` explains how to install tab
completion for bash, zsh, fish or PowerShell. `htt --version` prints
the version you have.

When you use htt at a terminal it checks GitHub, at most once a day,
for a newer release, and tells you if there is one. It never gets in
the way: offline or on any error it stays silent. The README's
[Online activity](README.md#online-activity) section says exactly what
it sends and how to turn it off (`check_for_updates: false`).

## Your first task

`htt todo add` (alias `htt t a`) appends an entry to the current
context. Out of the box the current context is `todo`. Every new task
is stamped with today's date as its creation date, todo.txt style.

```
$ htt todo add buy milk
Added: 2026-05-15 buy milk
```

List what's there with `htt todo show` (alias `htt t s`, or even just
`htt todo`):

```
$ htt todo

  0 2026-05-15 buy milk

(todo): 1 tasks
```

The leading `0` is the task's index in the current context — every
command that takes a task argument uses it.

## Priorities

`htt` follows [todo.txt](http://todotxt.org/) priority conventions —
single letters in parentheses, from `(A)` (highest) to `(C)` (lowest).
Set a priority on an existing task with `htt todo priority` (alias
`htt t p`):

```
$ htt todo add submit timesheet
Added: 2026-05-15 submit timesheet
$ htt todo priority 1 A
Before: 2026-05-15 submit timesheet
After:  (A) 2026-05-15 submit timesheet
```

`htt todo +` and `htt todo -` bump priority one step up or down (no
priority → `C` → `B` → `A`). Setting the priority re-sorted the file,
so `buy milk` is now task `1`:

```
$ htt todo + 1
Before: 2026-05-15 buy milk
After:  (C) 2026-05-15 buy milk
$ htt todo

  0 (A) 2026-05-15 submit timesheet
  1 (C) 2026-05-15 buy milk

(todo): 2 tasks
```

The file is sorted by priority after each priority change, so
re-listing shows the most urgent first — and indexes can move.

A task's first words have fixed meanings in todo.txt: `x` marks it
done, `(B)` is its priority, and a date is when it was created. If
taking away a priority would leave a task starting with one of those
(lowering `(C) x foo` would leave `x foo`, which reads as done), htt
puts today's date in front instead: `(C) x foo` becomes
`2026-11-17 x foo`, still open, still "x foo", and the TUI says so.
Removing an annotation from the start of a task works the same way.
Tasks htt added already start with their created date, so this only
touches tasks you wrote by hand.

## Completing tasks

`htt todo do <id>` (alias `htt t x`) marks the task complete, stamps it
with today's date and the context it came from, and moves it into the
`done` context:

```
$ htt todo do 1
Completed: x (C) 2026-05-15 2026-05-15 buy milk context:todo
```

The first date is when it was completed, the second when it was
created.

The `done` context is just another file — you can browse it like any
other:

```
$ htt todo context done
Now using context: done
$ htt todo

  0 x (C) 2026-05-15 2026-05-15 buy milk context:todo

(done): 1 tasks
$ htt todo context todo
Now using context: todo
```

## Contexts

Contexts are independent task lists — useful for separating work from
personal, or carving up by project. Switch with `htt todo context`
(alias `htt t c`); a new name starts an empty context:

```
$ htt todo context work
Now using context: work
$ htt todo add review PR for the new auth flow
Added: 2026-05-15 review PR for the new auth flow
$ htt todo add update on-call runbook
Added: 2026-05-15 update on-call runbook
$ htt todo

  0 2026-05-15 review PR for the new auth flow
  1 2026-05-15 update on-call runbook

(work): 2 tasks
```

`htt todo context` with no argument prints the current context, as
does `htt config context`.

Add to a named context without switching using `htt todo add-to`, and
move a task between contexts with `htt todo move` (alias `mv`):

```
$ htt todo add-to home pay credit card bill
Added: 2026-05-15 pay credit card bill to home
$ htt todo move 1 home
Moved 2026-05-15 update on-call runbook from work to home.
```

`htt todo status` (alias `htt t ?`) lists every context you can switch
to (everything except `done` and the archive) and shows the current
one's tasks:

```
$ htt todo status
Available Contexts: home todo work
Current Context: work

  0 2026-05-15 review PR for the new auth flow

(work): 1 tasks
```

Context names keep only letters, digits and underscores; anything
else becomes an underscore, and `htt` tells you the name it used:

```
$ htt todo context "weekend hacks!"
Now using context: weekend_hacks_
```

## Finding and changing tasks

`htt todo search` (aliases `query`, `q`) shows the tasks matching a
[Go regular expression](https://pkg.go.dev/regexp/syntax), matched
case-insensitively against the whole line — dates, priority and
annotations included:

```
$ htt todo search MILK

  0 2026-05-15 buy milk

(todo): 1 out of 3 tasks matched query.
```

Three ways to change a task:

- `htt todo replace <id> <new text>` (alias `r`) swaps the line for
  exactly what you type. It doesn't carry over the old priority or
  creation date:

  ```
  $ htt todo replace 0 buy oat milk
  Before: 2026-05-15 buy milk
  After: buy oat milk
  ```

- `htt todo edit <id>` (alias `e`) opens that one task in `$EDITOR`,
  then prints the same `Before:` / `After:` pair. Saving it unchanged
  or empty changes nothing.
- Editing the whole file by hand works too: each context is a plain
  text file (see [Where your data lives](#where-your-data-lives)).

`htt todo delete <id>` (alias `rm`) removes a task from its context.
It isn't lost: it moves to the archive, annotated with where it came
from and when:

```
$ htt todo delete 1
Deleted task: 2026-05-15 submit timesheet archived-from:todo deleted-on:2026-05-15
```

`htt todo edit-archive` (alias `ea`) opens the archive in `$EDITOR` if
you want to recover or prune entries, and `htt todo edit-done` (alias
`ed`) does the same for the done list. If `$EDITOR` isn't set, or the
editor won't start or exits with an error, these, `htt todo edit` and
`htt log edit` say so (`❌ $EDITOR variable is empty or not set`) and
exit with status 1.

When you can't decide what to do next, `htt todo random` prints one
task from the current context at random.

## Due dates

`htt todo add` (and `add-to`) accept a `--due` flag that takes either
an absolute date or a natural-language phrase. The phrase is resolved
to a date and stored as a `due:` annotation on the task — the file
always carries a structured date, regardless of how you typed it.

An annotation is any `key:value` word, wherever it sits in the line;
when htt rewrites a task (on a priority change or completion), every
word stays where you wrote it, annotations included, and new ones,
such as `context:` on completion, go at the end. Times (`10:30`,
`12:30pm`) and links (`https://…`, `mailto:…`, `tel:…`) look similar
but are text, not annotations.

```
$ htt todo add --due "next Friday" ship the auth refactor
Added: 2026-05-15 ship the auth refactor due:2026-05-22

$ htt todo add --due 2026-12-25 wrap presents
Added: 2026-05-15 wrap presents due:2026-12-25

$ htt todo add --due "in two weeks" review the migration plan
Added: 2026-05-15 review the migration plan due:2026-05-29
```

Phrases the parser understands include `Friday`, `next Monday`,
`tomorrow`, `in 3 days`, `in two weeks`, and absolute dates as
`YYYY-MM-DD`. A phrase the parser doesn't recognise surfaces as
an error rather than silently dropping the flag.

**Dates htt can't read.** Every line loads, whatever its dates, and
works as before. A date htt can't read is text: an impossible date
(`2026-13-45`, `2026-02-30`) or one not written `YYYY-MM-DD`
(`2026-5-3`) is just a word of the task, and a `due:` value that isn't
a `YYYY-MM-DD` date (`due:2026-02-30`, or `due:friday` typed into the
file by hand) leaves the task with no due date, so `overdue`,
`due-soon` and `--sort due` pass it by. htt points each one out on
stderr, naming the file and line:

```
$ htt todo show
warning: /home/you/.htt/data/todo.txt:2: 2026-13-45 isn't a real date, so it's text
warning: /home/you/.htt/data/todo.txt:3: due:friday isn't a YYYY-MM-DD date, so the task has no due date

  0 renew passport
  1 2026-13-45 call the bank
  2 pay rent due:friday

(todo): 3 tasks
```

A warning never changes what a command prints on stdout or its exit
status. Commands warn about the lists they read, once per line each
time they run: the current context for the `todo` commands, `status`
and `workon`, every list and `done.txt` for `htt report`, and once a
session in the REPL. Time logs are checked the same way (their
entries are tasks too): today's for the `log` commands but `log edit`,
for `status` and for `workon`, and each day's in its range for
`htt report`. The `ts:`
timestamp htt gives each entry never warns. A task you type with such a date warns as it's
written, naming the line it went to: `todo add`, `add-to`, `replace`
and `edit`, and `log add` for the time log. It's written all the
same:

```
$ htt todo add pay rent 2026-13-45
Added: 2026-05-15 pay rent 2026-13-45
warning: /home/you/.htt/data/todo.txt:4: 2026-13-45 isn't a real date, so it's text
```

Completing, deleting or moving a task doesn't warn about the list it
goes to, and the archive never warns: its tasks are deleted, and
nothing reads their dates. The TUI says it on its status line
instead.

## Sorting and filtering

`htt todo show` lists tasks in file order. Two flags turn it into a
view of the list instead:

- `--sort` orders by `priority`, `created` or `due`. A `-` prefix
  reverses it, so `--sort -created` puts the newest task first.
  Tasks with no created or due date stay at the bottom either way.
- `--filter` keeps only the tasks that match: `overdue`, `due-soon`
  (due in the next seven days), `no-due`, `has-tag:NAME` or
  `priority:A`/`B`/`C`. Repeat it to combine filters; a task has to
  match every one.

```
$ htt todo show --filter overdue --sort due

  4 (A) 2026-05-02 submit timesheet due:2026-05-14
  1 2026-05-08 review PR for the new auth flow due:2026-05-15

(work): 2 of 5 tasks shown.

$ htt todo show --filter has-tag:home

  2 2026-05-10 buy milk @home
  5 2026-05-12 fix the sink +home

(work): 2 of 5 tasks shown.
```

`has-tag:home` matches `@home`, `+home` and `#home`; write
`has-tag:+home` to match only the `+` kind.

The number beside each task is still its position in the full list,
so a sorted view can look out of order. That's deliberate:
`htt todo do 4` or `htt todo delete 4` acts on the task shown as `4`,
whatever order the view is in.

## Time logging

The activity log is a separate stream from todos — a record of what
you actually worked on, one file per day, each entry timestamped. An
entry means "from now on I'm doing this": its time runs until the
next entry.

- `htt log start` writes a `@start` entry.
- `htt log add <text>` (alias `htt log +`) logs what you're doing now.
- `htt workon <id>` (alias `wo`) logs a task from the current todo
  context, so you don't retype it.
- `htt log end` writes an `@end` entry, which ends the day.

Each prints the entry it logged:

```
$ htt log start
Logging entry: @start
$ htt log add reviewing the auth PR
Logging entry: reviewing the auth PR
$ htt todo

  0 2026-05-15 review PR for the new auth flow

(work): 1 tasks
$ htt workon 0
Logging entry: 2026-05-15 review PR for the new auth flow
```

`htt log show` (alias `s`) prints today's log. `htt log status`
(alias `htt log ?`) and `htt log active` both report the entry you're
on and how long it has been running:

```
$ htt log show
@start ts:2026-05-15T09:00:00Z
reviewing the auth PR ts:2026-05-15T09:00:12Z
2026-05-15 review PR for the new auth flow ts:2026-05-15T10:32:08Z

$ htt log status
Currently working on: 2026-05-15 review PR for the new auth flow (12m)
$ htt log active
Working on: 2026-05-15 review PR for the new auth flow (12m)
```

### Breaks and the end of the day

A break is an entry like any other:

```
$ htt log add @lunch
Logging entry: @lunch
```

Give it a category that doesn't count as work (see
[Categories](#categories)) and reports leave it out of the time you
worked, while still showing it.

`htt log end` ends the day: the entry before it stops there, and
`htt log status` says so:

```
$ htt log end
Logging entry: @end
$ htt log status
Not working (day ended at 17:00)
```

An entry logged after an `@end` starts again, and the time between
them counts as nothing, so a day where `log end` marked lunch still
adds up. A day with no `@end` counts its last entry until midnight,
and `htt report` flags it; today's last entry counts until now. Each
day stands alone, midnight to midnight in your time zone: an entry
still running at midnight stops there, and the next day starts with
its own first entry.

`htt log edit` (alias `e`) opens today's log file in `$EDITOR`, for
fixing a forgotten `log end` or a typo.

### Categories

Categories sort your time by the entries' todo.txt tags (`+project`,
`@context`, `#tag`), in the `timelog` section of the config file:

```yaml
timelog:
  categories:            # first match wins
    - name: break
      tags: ["@break", "@lunch"]
      work: false
    - name: htt
      tags: ["+htt"]
    - name: admin
      tags: ["@email", "+admin"]
  default_category: working
```

- An entry goes to the **first** category in the list with one of its
  tags, so `+htt @lunch` is a break. A tag can be in one category
  only.
- Tags match **exactly** as written: `@break` matches `@break coffee`,
  but not `@Break`, `@breakfast` or `@break-time`.
- A category counts as work unless it says `work: false`.
- Entries no category matches go to the **default category**:
  `working`, which counts as work, unless `default_category` names one
  of your categories, whose `work` then decides.
- `@start` is an ordinary entry: the time between `htt log start` and
  your first real entry is `working`, unless you give `@start` a
  category.
- `htt workon` logs the task's text, tags included, so a task's
  `+project` picks its category.
- A category that exists only to be the default needs no tags: give
  it a name (and `work: false` if its time shouldn't count) and name
  it in `default_category`. Any other category needs at least one tag,
  or nothing could ever be in it.

**Quote the tags**: in YAML an unquoted `#` starts a comment and an
unquoted `@` can't start a value. A config htt can't use stops
`htt report` with an error naming the key:

```
$ htt report
❌ config: timelog.categories: category 3 (admin): no tags, and it isn't the default category, so no entry can be in it; list its tags, or make it the default category
```

The log files themselves don't change: categories are read from the
tags each time, so changing them recounts past days too.

## Reports

`htt report` summarises a period: tasks completed, tasks added, tasks
deleted, and time logged per day. `--since` takes a date
(`YYYY-MM-DD`) or a duration counted back from now (`7d`, `2w`,
`24h`); the default is `7d`.

```
$ htt report --since 2d
Activity since 2026-05-13 (48h 0m)

Completed (1)
  todo:
    (C) buy milk                                       2026-05-15

Added (3)
  todo:
    (C) buy milk                                       2026-05-15
  work:
    review PR for the new auth flow                    2026-05-15
    update on-call runbook                             2026-05-15

Deleted (0)
  (none)

Time logged
  2026-05-13  14h 15m worked (no @end: counted to 24:00)
    break       45m
    htt      6h 10m
    admin     8h 0m
    working      5m
  2026-05-14    7h 0m worked
    break     1h 0m
    htt      4h 50m
    admin    1h 30m
    working     40m
  2026-05-15   1h 44m worked (in progress)
    working  1h 44m
  Total: 22h 59m worked
    break    1h 45m
    htt      11h 0m
    admin    9h 30m
    working  2h 29m
```

Time logged gives each day's worked time, then its time in each
category, breaks included, and the same totals for the period. A day
without an `@end` is counted to midnight and flagged; today's last
entry counts until now, in progress. A task added and completed in
the period counts under both Added and Completed, by the context it
was in; a deleted task is listed only under Deleted.

## The rollup

`htt status` (alias `htt ?`) is the one command that combines both:
current activity, every context, and the current context's tasks.
It's the natural "where am I?" command.

```
$ htt status
Currently working on: 2026-05-15 review PR for the new auth flow (24m)
Available Contexts: home todo work
Current Context: work

  0 2026-05-15 review PR for the new auth flow

(work): 1 tasks
```

## Where your data lives

```
~/.config/htt/config.yaml          ← config (see Configuration below)
~/.config/htt/repl-history         ← commands typed in `htt repl`
~/.htt/data/                       ← all task files
  current-context                  ← name of the active context
  todo.txt                         ← one file per context
  work.txt
  home.txt
  done.txt                         ← completed tasks
  archive.txt                      ← deleted tasks
  *.txt.bak                        ← the previous version of each file
  timelogs/                        ← daily activity logs
    2026-05-15.log
    2026-05-14.log
    …
~/.cache/htt/update-check.json     ← when htt last checked for a new release
```

Each context file is plain todo.txt format — one task per line, no
header. You can edit them by hand; `htt` rewrites a file only when a
command changes it, keeping the previous version as `.bak`. Anything
else in the data directory, such as a `.git/` if you keep it under
version control, htt leaves alone.

`htt config where-data` prints the data directory if you ever forget
where it lives.

## Configuration

htt reads `config.yaml` from the directory you run it in, if there is
one, and otherwise from `~/.config/htt/`. A `config.yaml` in your
current directory therefore overrides your normal settings — handy for
a throwaway task list, surprising if you didn't expect it.

`htt config yaml` prints the settings in effect. The keys:

| Key                   | Default                         | What it's for                                         |
| --------------------- | ------------------------------- | ----------------------------------------------------- |
| `data_path`           | `~/.htt/data`                   | where the context, done, archive and timelog files live |
| `tracker_path`        | `~/.htt/data`                   | where the `current-context` pointer lives             |
| `config_path`         | `~/.config/htt/config.yaml`     | the config file itself (informational)                |
| `no-color`            | `false`                         | `true` does what `--no-color` does, every time        |
| `tui.theme`           | `default`                       | the TUI's colour theme (see [Colours and themes](#colours-and-themes)) |
| `tui.colors`          | unset                           | per-colour overrides on top of the theme              |
| `check_for_updates`   | `true`                          | check GitHub for a newer release, at most once a day (see [Online activity](README.md#online-activity)) |
| `timelog.categories`  | unset                           | categories for time-log entries, by tag (see [Categories](#categories)) |
| `timelog.default_category` | `working`                 | the category of entries no category matches (see [Categories](#categories)) |

The `config` subcommands print single values: `htt config where-data`
(the data directory), `htt config dir` (the config directory, the one
holding the `config.yaml` in effect: `~/.config/htt`, or the current
directory when htt is reading a `config.yaml` there) and `htt config
context` (the current context).

## Optional: interactive mode

`htt interactive` (or `htt i`) launches a full-screen TUI over the same
files. The keys:

| Key                  | Does                                            |
| -------------------- | ----------------------------------------------- |
| `↑`/`k`, `↓`/`j`     | move between tasks                              |
| `PgUp` / `PgDn`      | previous / next page of a long list             |
| `g` / `G` (`Home`/`End`) | first / last task                           |
| `←`/`h`, `→`/`l`     | switch context tab                              |
| `/`                  | search (see below); `Enter` applies it, `Esc` clears it |
| `x`                  | complete the task                               |
| `d`                  | delete the task (it moves to the archive)       |
| `+` / `-`            | raise / lower its priority                      |
| `n` or `a`           | add a task: type it, `Enter` to add, `Esc` to cancel |
| `e`                  | edit the task: change it, `Enter` to save, `Esc` to cancel |
| `f`                  | edit the current tab's file in `$EDITOR`        |
| `?`                  | show or hide the full keymap                    |
| `q`, `Esc`, `Ctrl-C` | quit (`Esc` clears a search first, if there is one) |

The keymap also shows `:`, reserved for a command mode that doesn't
do anything yet.

**Searching.** `/` opens a search box above the list, which narrows as
you type. Each word is either one of the filters `htt todo show
--filter` takes (`overdue`, `due-soon`, `no-due`, `has-tag:NAME`,
`priority:A`) or text, matched loosely against the whole task line
(`mlk` finds "milk"; case doesn't matter). A task has to match every
word: `priority:A home` finds priority-A tasks mentioning "home".
`Enter` applies the search, and the line above the list then shows it
(`search: priority:A home · 2 of 12 (esc clears)`); the characters the
text words matched are underlined. Tasks keep their
numbers in a search, and everything you do acts on the task under the
cursor, so `x` completes exactly the task you see. A search stays in
place when you complete, delete or reprioritise a task, and when you
switch tabs; `Esc` clears it. `Enter` on a search that matches nothing
clears it.

Changing a task's priority re-sorts the file; the cursor follows the
task to its new place.

**Adding and editing.** `n` opens an empty task input; `e` opens the
task under the cursor in it, to change any part of the line. As you
type, the line below shows the task exactly as it will be saved, in
the list's colours: a new task gets today's date after any priority,
as `htt todo add` gives it. An edit is saved where the task was, so
the file keeps its order.

- **Due dates in words.** `due:friday`, `due:tomorrow` or, quoted,
  `due:"in two weeks"` are saved as `due:YYYY-MM-DD`, read the same
  way as `htt todo add --due` (a hyphen also works:
  `due:next-friday`). A due date htt can't read stops the save, and
  the line below says why; nothing you typed is lost.
- **Tags.** Start a tag (`+`, `@` or `#`) and press `Tab` to complete
  it from the tags you've used in any tab, most used first;
  `Ctrl-N` / `Ctrl-P` step through the others.
- **Notes.** The input also points out words that won't mean what
  they look like: `(a)` (priorities are capitals), `(B)` after the
  start of the task (it's text there), a priority other than A, B or
  C, a date htt can't read like `2026-13-45` or `2026-5-3` (see
  [Dates htt can't read](#due-dates)), or an `x` at the start of a
  new task (it marks the task done). These save as typed.

Tasks in the done tab can't be added or edited here; `f` opens
`done.txt` if you need to.

`f` hands the terminal to `$EDITOR` and reloads the file when the
editor exits. If `$EDITOR` isn't set, or the editor won't start or
exits with an error, the TUI says so on a line above the keymap and
carries on; after an editor error it still reloads, in case the file
was saved.

When a tab shows tasks with dates htt can't read (see
[Dates htt can't read](#due-dates)), the same line says so for the
first, with a count of the rest: `Task 1: 2026-13-45 isn't a real
date, so it's text (and 1 more date warning in this list).` Each
line is mentioned once a session.

The TUI works on the same data files as the CLI — you can switch
between them in the same session without anything getting out of sync.

### Colours and themes

The TUI's colours come from a theme, set in the `tui` section of the
config file. The built-in themes are `default` (htt's own palette),
`monokai`, `vscode` (VS Code's Dark+), `vscode-light` (Light+),
`dracula`, `nord`, `solarized-dark`, `solarized-light` and `gruvbox`;
`htt interactive --help` lists them too.

```yaml
tui:
  theme: monokai
```

Override any of the theme's colours under `colors`, by role:

```yaml
tui:
  theme: nord
  colors:
    accent: "#ebcb8b"
    selection: "#5e81ac"
```

| Role         | Colours                                                              |
| ------------ | -------------------------------------------------------------------- |
| `foreground` | the `htt` title, text on the selected tab and task, and the fill of the done tab and the new-task input |
| `background` | text drawn on that fill                                              |
| `selection`  | the selected tab and task                                            |
| `accent`     | the new-task prompt and the status line                              |
| `muted`      | borders, keys in the help bar, the new-task placeholder              |
| `help`       | descriptions in the help bar                                         |
| `separator`  | separators in the help bar                                           |
| `priority_a`, `priority_b`, `priority_c` | a task's priority: `(A)`, `(B)`, `(C)`   |
| `plus_tag`, `at_tag`, `hash_tag` | tags: `+garden`, `@phone`, `#idea`               |
| `annotation` | `key:value` annotations, such as `due:2026-05-20` (a time like `10:30` or a link like `https://…` is text) |
| `date`       | the created-on and completed-on dates at the start of a task         |

The task-line colours (the last four rows) paint every task row but
the selected one, which keeps `foreground` on `selection`. A word
gets its colour only where todo.txt gives it that meaning: `(A)` is a
priority at the start of a line, but just text after it. While you
search, the characters the search matched are underlined.

A colour is a hex value (`"#rrggbb"` or `"#rgb"`) or an ANSI colour
number (`0`–`255`). **Quote hex values**: unquoted, YAML reads
everything from the `#` on as a comment. A misspelt theme or role, or
a colour htt can't read, stops `htt interactive` with an error naming
the valid choices.

The TUI doesn't paint the whole screen: the background, tabs that
aren't selected, and the plain words of tasks keep your terminal's
own colours. Pick a theme that suits your terminal; a light theme on a
dark terminal leaves the title hard to read. Colours are adapted to
what your terminal supports (true colour, 256 or 16 colours).

The TUI writes nothing outside its own screen. If something goes
wrong (say it closes unexpectedly), run it with `--log-file` and it
appends what it's doing to that file. The log is written at **debug**
level, the most detailed, unless you pick another with `--log-level`:

| `--log-level` | What gets logged                                        |
| ------------- | ------------------------------------------------------- |
| `debug`       | every action the TUI runs (each keypress)               |
| `info`        | the TUI starting and exiting                            |
| `warn`        | an action that can't run right now, like completing a task in the done tab or editing with no `$EDITOR` set |
| `error`       | an action or the editor failed, with the reason; most action failures also close the TUI, editor failures don't |

Each level also includes the levels below it in the table.

```
$ htt interactive --log-file ~/htt-tui.log
# in another shell:
$ tail -f ~/htt-tui.log
time=2026-11-02T10:15:02.114Z level=INFO msg="tui started" context=todo
time=2026-11-02T10:15:04.870Z level=DEBUG msg=action action="move down"
time=2026-11-02T10:15:05.392Z level=DEBUG msg=action action=do
```

`--log-level` needs `--log-file`; on its own it's an error.

## Optional: REPL mode

`htt repl` opens a todo-mode shell — handy for a focused pass over
your tasks without retyping `htt todo` on every line. It shows the
current context, then prompts. Each command clears the screen, then
shows the line you typed, the command's output, and the context again
with the command's effect:

```
htt(work)> add update on-call runbook
Added: 2026-05-15 update on-call runbook

  0 (A) 2026-05-15 ship the auth refactor
  1 2026-05-15 review PR for the new auth flow
  2 2026-05-15 update on-call runbook

(work): 3 tasks
htt(work)>
```

The output stays on screen until your next command. If it and the
list together are taller than the terminal, the top of the output
scrolls back, as in any shell.

Commands without a prefix dispatch as `todo <cmd>`: `add foo`,
`delete 0`, `+ 1`, `do 0`, `context home`. Prefix a line with `/`
to escape to a full CLI command: `/log start`, `/report --since 7d`.
`/interactive` and `/repl` are disabled inside the REPL (no nested
screens / recursive prompts). `/help` shows the REPL's own help.

Exit with Ctrl-D, Ctrl-C, or by typing `quit`, `exit`, or `q`. An
empty Enter clears the last output and redraws the context. Up/down
arrows walk the commands you've typed, earlier sessions included:
they're kept in `repl-history` in the config directory
(`htt config dir`). Warnings about dates htt can't read show once a
session.

## Where to go next

- `htt help <command>` for the full flag list on any subcommand.
- [`docs/architecture.md`](docs/architecture.md) for the package
  layout and how to add features.
- [`TODO.md`](TODO.md) for the wishlist and known issues, and
  [GitHub Issues](https://github.com/bricef/htt/issues) for scoped
  work — happy to take pull requests.
