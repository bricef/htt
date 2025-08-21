# Hypothetical Tasks & Time Tracker (htt)

## About

`htt` is a [Todo.txt](http://todotxt.org/) compatible command line todo list manager and time tracker.

## Installation

If you have Go properly installed and configured, you may install the client using `go get`:

```
go install github.com/bricef/htt/cmd/htt
```

Which will make the `htt` command available.

## Usage

```
$ htt help
htt is a command line todo list manager and time tracker

Usage:
  htt [flags]
  htt [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  config      Manage configuration.
  help        Help about any command
  log         Manage the activity log.
  status      Show the status of the tasklist and time log.
  sync        Sync the data to the backend manually.
  todo        Manage todo lists.
  workon      Log that work has began on numbered item in the current context.

Flags:
  -h, --help       help for htt
      --no-color   Disable color output for coloured commands

Use "htt [command] --help" for more information about a command.
```

## Tip 

To make managing logs and todo easier, it might be worth adding the following aliases (or similar) to your terminal:

```shell
alias t="htt t"
alias jnl="htt l"
```

## Todo
- Interactive Mode
  - [x] Set up interactive mode with Bubbletea, bubbles and lipgloss
  - [ ] Enable task action from interactive UI
  - [ ] Highlight tags KV and dates in task render
  - [ ] Enable command mode
- QA
  - [ ] Write intergation tests for htt. Make sure that mutating actions work as intended
- Repos
  - [ ] Check if repo exists. If it doesn't clone it first
  - [ ] Handle case when local repo is behind remote. (Rebase? Merge? How?)
  - [ ] Add 'status' action which shows sync status
  - [ ] Commit on all modifications.
- UX
  - [ ] Enable managing contexts (archive, delete, merge)
  - [ ] Change delete from removal to archival in separate file.
  - [ ] Add `due:Friday` smart parsing
  - [ ] Add `due:"in two weeks"` smart parsing
  - [ ] Enable showing in priority order
  - [ ] Add deduplicate command (smart with edit distance?)
  - [ ] Add "log@" command to log at a particular time
  - [ ] Review jrnl command to see if we can take inspiration from this
  - [ ] Add "Where is <>?" command
  - [ ] archive context
  - [ ] archive todo
  - [ ] Query language
  - [ ] interval output
  - [ ] set up "htt do context/line" command to complete tasks across contexts
- [ ] Fork goparsec to fix messed up API choices
  - AST/Node distinction? 
  - Simple querying

## Production grade

- [ ] Set up packaging for release. (see https://github.com/goreleaser/goreleaser)
- [ ] Set up CI

## Future

- [ ] Set up interactive cli (see https://github.com/c-bata/go-prompt)
- UI. Either:
  - [ ] Set up a CLI GUI like todotxt-machine (using https://github.com/jroimartin/gocui or https://github.com/gizak/termui) 
  - [ ] Delegate full editing to other program (https://github.com/AnthonyDiGirolamo/todotxt-machine)
- [ ] Create a GUI app layer (https://github.com/avelino/awesome-go#gui, https://github.com/murlokswarm/app, )

## Done

- [x] Add "summary" command to show tasks and prioritoes across contexts
- [x] Action: increase priority
- [x] Action: decrease priority
- [x] Add 'do' action
- [x] Add 'Add to' action
- [x] Add 'switch' action (context alias)
- [x] Enable short commands
- [x] Add 'work on' action
- [x] Add 'add+workon' action ('log' ?)
- [x] Show active and other contexts on `show`
- [x] Create a todotxt line parser (https://github.com/todotxt/todo.txt)
  - https://github.com/alecthomas/participle
  - https://github.com/mna/pigeon
  - https://github.com/prataprc/goparsec
  - https://github.com/vektah/goparsify
  - https://github.com/pointlander/peg

## Prior art
- https://github.com/google/go-github (Github API client)
- https://github.com/gammons/todolist Also in Go, similar idea with good query parsing
- Command line journaling: http://jrnl.sh/
- https://github.com/VladimirMarkelov/ttdl (terminal Todo List Manager in Rust)
- https://github.com/hugokernel/todofi.sh (todotxt with [Rofi](https://github.com/davatorium/rofi))
- https://xwmx.github.io/nb/
- https://github.com/yuucu/todotui
- https://codeberg.org/pter/pter
- https://github.com/Fanteria/todotxt-tui
- http://todotxt.org/
- https://www.solidtime.io/ / https://github.com/solidtime-io/solidtime

