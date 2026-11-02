# Getting started with htt

A fifteen-minute tour. By the end you'll have added a few tasks,
prioritised them, completed one, organised the rest into contexts,
logged some work against them, and seen the rollup status.

If you just want the install + reference, [`README.md`](README.md) has
the quick version. This doc walks through a realistic workflow.

## Install

With a working Go toolchain:

```
$ go install github.com/bricef/htt/cmd/htt@latest
$ htt --help
```

The first run creates `~/.config/htt/config.yaml` with sensible
defaults. Your task files will live under `~/.htt/data/`.

```
$ htt
Could not find a configuration file.
Creating a default file at /home/you/.config/htt/config.yaml.
htt is a command line todo list manager and time tracker
…
```

## Your first task

`htt todo add` (alias `htt t a`) appends an entry to the current
context. Out of the box the current context is `todo`.

```
$ htt todo add buy milk
Added: buy milk
```

List what's there with `htt todo show` (alias `htt t s`, or even just
`htt todo`):

```
$ htt todo

  0 buy milk

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
Added: submit timesheet
$ htt todo
  0 buy milk
  1 submit timesheet
(todo): 2 tasks

$ htt todo priority 1 A
Before: submit timesheet
After:  (A) submit timesheet
```

`+` and `-` bump priority one step up or down:

```
$ htt todo + 0
Before: buy milk
After:  (C) buy milk
$ htt todo
  0 (A) submit timesheet
  1 (C) buy milk
(todo): 2 tasks
```

The on-disk file is sorted by priority after each priority change, so
re-listing always shows the most urgent first.

## Completing tasks

`htt todo do <id>` (alias `htt t x`) marks the indexed task complete,
stamps it with today's date and the source context, and moves it into
the `done` context.

```
$ htt todo do 1
Completed: x 2026-05-15 buy milk context:todo
```

The `done` context is just another file — you can browse it like any
other:

```
$ htt todo context done
Now using context: done
$ htt todo
  0 x 2026-05-15 buy milk context:todo
(done): 1 tasks
$ htt todo context todo
Now using context: todo
```

## Contexts

Contexts are independent task lists — useful for separating work from
personal, or carving up by project. Switch with `htt todo context`
(alias `htt t c`):

```
$ htt todo context work
Now using context: work
$ htt todo add review PR for the new auth flow
Added: review PR for the new auth flow
$ htt todo add update on-call runbook
Added: update on-call runbook
$ htt todo
  0 review PR for the new auth flow
  1 update on-call runbook
(work): 2 tasks
```

Add to a named context without switching using `htt todo add-to`:

```
$ htt todo add-to home pay credit card bill
Added: pay credit card bill to home
```

## Due dates

`htt todo add` (and `add-to`) accept a `--due` flag that takes either
an absolute date or a natural-language phrase. The phrase is resolved
to a date and stored as a `due:` annotation on the task — the file
always carries a structured date, regardless of how you typed it.

```
$ htt todo add --due "next Friday" ship the auth refactor
Added: 2026-05-22 ship the auth refactor due:2026-05-29

$ htt todo add --due 2026-12-25 wrap presents
Added: 2026-05-22 wrap presents due:2026-12-25

$ htt todo add --due "in two weeks" review the migration plan
Added: 2026-05-22 review the migration plan due:2026-06-05
```

Phrases the parser understands include `Friday`, `next Monday`,
`tomorrow`, `in 3 days`, `in two weeks`, and absolute dates as
`YYYY-MM-DD`. A phrase the parser doesn't recognise surfaces as
an error rather than silently dropping the flag.

`htt todo status` (alias `htt t ?`) lists every available context and
shows the current one's tasks:

```
$ htt todo status
Available Contexts: home todo work
Current Context: work

  0 review PR for the new auth flow
  1 update on-call runbook

(work): 2 tasks
```

A name with characters that aren't valid in filenames (spaces,
punctuation, …) is sanitised — `htt todo context "with spaces!"`
persists as `with_spaces_`, and `htt` tells you what it actually used:

```
$ htt todo context "weekend hacks"
Now using context: weekend_hacks
```

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

The activity log is a separate stream from todos — it's an append-only
record of what you actually worked on, with timestamps.

`htt log start` marks the beginning of a work session; `htt log add`
appends a freeform note about the current activity; `htt log end`
marks the end.

```
$ htt log start
Logging entry: @start
$ htt log add reviewing the auth PR
Logging entry: reviewing the auth PR
$ htt log add answered support ticket #4123
Logging entry: answered support ticket #4123
```

`htt workon <id>` is a shortcut: it pulls the indexed task from the
current todo context and logs it as the current activity. Useful when
the thing you're starting is already in your todo list.

```
$ htt todo
  0 review PR for the new auth flow
  1 update on-call runbook
(work): 2 tasks
$ htt workon 0
Logging entry: review PR for the new auth flow
```

`htt log show` prints today's log; `htt log status` (alias `htt log ?`)
prints the current activity and how long you've been on it:

```
$ htt log show
@start ts:2026-05-15T09:00:00Z
reviewing the auth PR ts:2026-05-15T09:00:12Z
answered support ticket #4123 ts:2026-05-15T10:14:33Z
review PR for the new auth flow ts:2026-05-15T10:32:08Z

$ htt log status
Currently working on: review PR for the new auth flow (12m)
```

When you stop for the day:

```
$ htt log end
Logging entry: @end
```

A note on `@end`: the active-task logic doesn't currently treat it as
a sentinel. After `htt log end`, `htt log status` will report
"Currently working on: @end (3m)". That's a known wart — see
[`TODO.md`](TODO.md) — and a stricter Open/Closed model is on the
list.

## The rollup

`htt status` (alias `htt ?`) is the one command that combines both:
current activity, every context, and the current context's tasks.
It's the natural "where am I?" command.

```
$ htt status
Currently working on: review PR for the new auth flow (24m)
Available Contexts: home todo work
Current Context: work

  0 review PR for the new auth flow
  1 update on-call runbook

(work): 2 tasks
```

## Where your data lives

```
~/.config/htt/config.yaml          ← config (data path, etc.)
~/.htt/data/                       ← all task files
  current-context                  ← name of the active context
  todo.txt                         ← one file per context
  work.txt
  home.txt
  done.txt                         ← completed tasks
  archive.txt                      ← deleted tasks (manual GC)
  timelogs/                        ← daily activity logs
    2026-05-15.log
    2026-05-14.log
    …
```

Each context file is plain todo.txt format — one task per line, no
header. You can edit them by hand if you want; `htt todo edit-done`
opens the done file and `htt todo edit-archive` opens the archive
file in `$EDITOR`. Deleted tasks are not lost — they move to
`archive.txt` annotated with `archived-from:` and `deleted-on:`,
visible in `htt report`'s Deleted section.

`htt config where-data` prints the resolved data directory if you
ever forget where it lives.

## Optional: interactive mode

`htt interactive` (or `htt i`) launches the bubble tea TUI: arrow
keys navigate tasks, `x` completes, `d` deletes, `+`/`-` change
priority, `n`/`a` add a new task, `h`/`l` switch contexts, `?` toggles
the keymap help. `q` or `esc` quits.

The TUI works on the same data files as the CLI — you can switch
between them in the same session without anything getting out of sync.

The TUI writes nothing outside its own screen. If something goes
wrong (say it closes unexpectedly), run it with `--log-file` and it
appends what it's doing to that file. The log is written at **debug**
level, the most detailed, unless you pick another with `--log-level`:

| `--log-level` | What gets logged                                        |
| ------------- | ------------------------------------------------------- |
| `debug`       | every action the TUI runs (each keypress)               |
| `info`        | the TUI starting and exiting                            |
| `warn`        | an action that doesn't apply where you are, like completing a task in the done tab |
| `error`       | an action that failed, with the reason; most failures also close the TUI |

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
your tasks without retyping `htt todo` on every line. Each cycle
clears the screen, shows the current context, then prompts:

```
(work)
  0 (A) ship the auth refactor
  1 review PR for the new auth flow
(work): 2 tasks

htt(work)> add update on-call runbook
Added: update on-call runbook
htt(work)>
```

Commands without a prefix dispatch as `todo <cmd>`: `add foo`,
`delete 0`, `+ 1`, `do 0`, `context home`. Prefix a line with `/`
to escape to a full CLI command: `/log start review PRs`,
`/report --since 7d`, `/sync`. `/interactive` and `/repl` are
disabled inside the REPL (no nested screens / recursive prompts).

Exit with Ctrl-D, Ctrl-C, or by typing `quit`, `exit`, or `q`. An
empty Enter refreshes the view. Up/down arrows walk command
history (persisted across sessions under your config dir).

## Optional: git sync

`htt sync` commits the data directory into a local git repo and
pushes to whatever remote is configured in
`~/.config/htt/config.yaml` (`backing_repo_url` + `remote_name`). This
is the easiest way to keep one task list across machines: configure
the same remote on each machine, run `htt sync` after a change, run
`htt sync` again at the new machine to pull.

If you don't configure a remote, `htt sync` is a no-op and you can
ignore the command entirely.

## Where to go next

- `htt help <command>` for the full flag list on any subcommand.
- [`docs/architecture.md`](docs/architecture.md) for the package
  layout and how to add features.
- [`TODO.md`](TODO.md) for the open wishlist — happy to take pull
  requests.
