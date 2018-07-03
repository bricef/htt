# Hypothetical Tasks & Time Tracker (htt)

## Installation

If you have Go properly installed and configured, you may install the client using `go get`:

```
go get github.com/hypotheticalco/tracker-client/htt
```

Which will make the `htt` command available.

## Usage

```shell
$ htt help
Hypothetical Tasks & Time Tracker is a todo list manager and time tracker

Usage:
  htt [flags]
  htt [command]

Available Commands:
  add            Add an item to the current tasklist
  addLog         Add an entry to the current tasklist and immediately start working on it.
  addTo          Add an item to a specified tasklist
  context        Change the context for tasks
  currentContext Outputs the current context
  dataDir        Outputs the currently configured datadir
  delete         Delete the item specified
  do             Complete a task
  edit           Edit the item specified using $EDITOR
  help           Help about any command
  log            Log an entry to the time log.
  replace        Replace an item with a new entry
  show           Show the default tasklist.
  sync           Sync the data to the backend manually
  workon         Log that work has began on numbered item.

Flags:
  -h, --help   help for htt

Use "htt [command] --help" for more information about a command.
```

## Todo

- Repos
  - [ ] Check if repo exists. If it doesn't clone it first
  - [ ] Handle case when local repo is behind remote. (Rebase? Merge? How?)
  - [ ] Add 'status' action which shows sync status
- UX
  - [ ] deduplicate
  - [ ] increase priority
  - [ ] decrease priority
  - [ ] report
- [ ] Create a todotxt line parser (https://github.com/todotxt/todo.txt)
  - https://github.com/alecthomas/participle
  - https://github.com/mna/pigeon
  - https://github.com/prataprc/goparsec
  - https://github.com/vektah/goparsify
  - https://github.com/pointlander/peg

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

- [x] Add 'do' action
- [x] Add 'Add to' action
- [x] Add 'switch' action (context alias)
- [x] Enable short commands
- [x] Add 'work on' action
- [x] Add 'add+workon' action ('log' ?)
- [x] Show active and other contexts on `show`
