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
completion for bash, zsh, fish or PowerShell.

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
`ed`) does the same for the done list.

When you can't decide what to do next, `htt todo random` prints one
task from the current context at random.

## Due dates

`htt todo add` (and `add-to`) accept a `--due` flag that takes either
an absolute date or a natural-language phrase. The phrase is resolved
to a date and stored as a `due:` annotation on the task — the file
always carries a structured date, regardless of how you typed it.

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
you actually worked on, one file per day, each entry timestamped.

- `htt log start` writes a `@start` entry.
- `htt log add <text>` (alias `htt log +`) logs what you're doing now.
- `htt workon <id>` (alias `wo`) logs a task from the current todo
  context, so you don't retype it.
- `htt log end` writes an `@end` entry.

Only `log add` prints anything; the others write their entry silently
(see [`TODO.md`](TODO.md#known-issues)).

```
$ htt log start
$ htt log add reviewing the auth PR
Logging entry: reviewing the auth PR
$ htt todo

  0 2026-05-15 review PR for the new auth flow

(work): 1 tasks
$ htt workon 0
```

`htt log show` (alias `s`) prints today's log. `htt log status`
(alias `htt log ?`) and `htt log active` both report the latest entry
and how long ago it started:

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

When you stop for the day:

```
$ htt log end
```

A note on `@end`: it's an entry like any other, so after
`htt log end`, `htt log status` reports
"Currently working on: @end (3m)". That's a known wart (listed in
[`TODO.md`](TODO.md#known-issues)); a stricter open/closed model for
entries is on the wishlist.

`htt log edit` (alias `e`) opens today's log file in `$EDITOR`, for
fixing a forgotten `log end` or a typo.

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
  2026-05-15  1h 32m
  Total: 1h 32m (excluding currently-active entry)
```

Time is counted between consecutive log entries, so the entry you're
still on isn't included. One known issue: a task you've deleted is
also listed under Added, as if from a context named `archive`.

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
  .git/                            ← only after you first run htt sync
```

Each context file is plain todo.txt format — one task per line, no
header. You can edit them by hand; `htt` rewrites a file only when a
command changes it, keeping the previous version as `.bak`.

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
| `backing_repo_url`    | unset                           | sync: the SSH remote to push to                       |
| `remote_name`         | `origin`                        | sync: the name given to that remote                   |
| `ssh_key_file`        | `~/.ssh/id_rsa.pub`             | sync: the private key to push with (the default won't work; see below) |
| `author_name`         | `HTT`                           | sync: the commit author                               |
| `author_email`        | `htt@fractallambda.com`         | sync: the commit author's email                       |
| `file_match_patterns` | `*.txt`, `.context`, `timelogs`, `.gitignore` | sync: which files get committed     |

The `config` subcommands print single values: `htt config where-data`
(the data directory), `htt config dir` (the tracker directory — despite
the name, not the config directory) and `htt config context` (the
current context).

## Optional: interactive mode

`htt interactive` (or `htt i`) launches a full-screen TUI over the same
files. The keys:

| Key                  | Does                                            |
| -------------------- | ----------------------------------------------- |
| `↑`/`k`, `↓`/`j`     | move between tasks                              |
| `←`/`h`, `→`/`l`     | switch context tab                              |
| `x`                  | complete the task                               |
| `d`                  | delete the task (it moves to the archive)       |
| `+` / `-`            | raise / lower its priority                      |
| `n` or `a`           | add a task: type it, `Enter` to add, `Esc` to cancel |
| `f`                  | edit the current tab's file in `$EDITOR`        |
| `?`                  | show or hide the full keymap                    |
| `q`, `Esc`, `Ctrl-C` | quit                                            |

The keymap also shows `:`, reserved for a command mode that doesn't
do anything yet.

`f` hands the terminal to `$EDITOR` and reloads the file when the
editor exits. If `$EDITOR` isn't set, or the editor won't start or
exits with an error, the TUI says so on a line above the keymap and
carries on; after an editor error it still reloads, in case the file
was saved.

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
your tasks without retyping `htt todo` on every line. Each cycle
clears the screen, shows the current context, then prompts:

```

  0 (A) 2026-05-15 ship the auth refactor
  1 2026-05-15 review PR for the new auth flow

(work): 2 tasks
htt(work)> add update on-call runbook
```

Commands without a prefix dispatch as `todo <cmd>`: `add foo`,
`delete 0`, `+ 1`, `do 0`, `context home`. Prefix a line with `/`
to escape to a full CLI command: `/log start`, `/report --since 7d`.
`/interactive` and `/repl` are disabled inside the REPL (no nested
screens / recursive prompts). `/help` shows the REPL's own help.

Exit with Ctrl-D, Ctrl-C, or by typing `quit`, `exit`, or `q`. An
empty Enter refreshes the view. Up/down arrows walk the commands
you've typed this session.

Two known issues (see [`TODO.md`](TODO.md#known-issues)): command
history isn't saved between sessions, and because the screen is
cleared as soon as a command finishes, a command's own output
(`Added: …`, search results, `/report`) disappears straight away —
your terminal's scrollback may still have it. The REPL suits changing
tasks more than reading output.

## Optional: git sync

`htt sync` commits your data directory to git and pushes it to a
remote. It's manual and push-only: nothing syncs unless you run it,
and it never pulls. Its problems and limits are tracked in
[#12](https://github.com/bricef/htt/issues/12).

> **Set `backing_repo_url` before you first run `htt sync`.** Without
> it, sync pushes your data into whatever git repository the current
> directory belongs to.

What a sync does:

1. Makes the data directory a git repository (branch `master`) if it
   isn't one yet.
2. Points the remote `remote_name` (default `origin`) at
   `backing_repo_url`.
3. Commits the files matching `file_match_patterns`: every context
   file, `done.txt`, `archive.txt` and `timelogs/`. The
   `current-context` pointer and `.bak` files aren't committed, so
   each machine keeps its own current context.
4. Pushes over SSH, as user `git`, with the private key in
   `ssh_key_file`.

To set it up, create an empty repository (no README or licence, so
htt's `master` is its only branch) and point the config at it:

```yaml
backing_repo_url: git@github.com:you/tasks.git
ssh_key_file: /home/you/.ssh/id_ed25519
author_name: You
author_email: you@example.com
```

The remote has to be SSH (HTTPS remotes fail with
`invalid auth method`), and the key has to be a private key without a
passphrase (there's no ssh-agent support). The default `ssh_key_file`,
`~/.ssh/id_rsa.pub`, is a public key and never works.

```
$ htt sync
2026/05/15 09:30:12 ℹ️  Could not open repository. Attempting to create one instead.
2026/05/15 09:30:12 ✅  Created commit of changes with hash  a9a6bdf6…
2026/05/15 09:30:13 ✅  Pushed commit a9a6bdf6… to origin
```

**A second machine.** Clone the backing repository into the data
directory before running `htt sync` there (move any existing files
out of the way first):

```
$ git clone git@github.com:you/tasks.git ~/.htt/data
```

A machine that syncs from its own fresh data directory instead has its
push rejected (`non-fast-forward update`) and never sees the other
machine's tasks.

**When another machine has pushed.** `htt sync` commits locally, then
the push is rejected and it exits with an error. Bring the data
directory up to date with git, then sync again:

```
$ git -C ~/.htt/data pull --no-rebase origin master
$ htt sync
```

(Spell out `origin master`: on the machine where `htt sync` created
the repository, `master` has no upstream set, so a bare `git pull`
doesn't know where to pull from.)

If both machines changed the same context since the last sync, git
reports a merge conflict in that file. The files are plain text, one
task per line, so resolve it as you would any conflict, commit, then
sync.

Every failed sync still leaves its local commit behind, and any error
ends the command straight away.

## Where to go next

- `htt help <command>` for the full flag list on any subcommand.
- [`docs/architecture.md`](docs/architecture.md) for the package
  layout and how to add features.
- [`TODO.md`](TODO.md) for the wishlist and known issues, and
  [GitHub Issues](https://github.com/bricef/htt/issues) for scoped
  work — happy to take pull requests.
