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
  config         Prints out the current configuration in YAML
  context        Change the context for tasks
  currentContext Outputs the current context
  dataDir        Outputs the currently configured datadir
  delete         Delete the item specified
  do             Complete a task
  edit           Edit the item specified using $EDITOR
  editDone       Open the done file using $EDITOR
  editLog        Open the current time log file using $EDITOR
  get            Show tasks across lists.
  help           Help about any command
  log            Log an entry to the time log.
  pri+           increase the priority for the selected task
  pri-           Decrease the priority for the selected task
  random         Select an item at random from the tasklist
  replace        Replace an item with a new entry
  show           Show the default tasklist.
  showLog        Show the day's time log.
  status         Show the status of the tasklist and time log
  sync           Sync the data to the backend manually
  workingon      Show the current active time log entry.
  workon         Log that work has began on numbered item.

Flags:
  -h, --help       help for htt
      --no-color   Disable color output for coloured commands

Use "htt [command] --help" for more information about a command.
```

## Todo
- QA
  - [ ] Write intergation tests for htt. Make sure that mutating actions work as intended
- Repos
  - [ ] Check if repo exists. If it doesn't clone it first
  - [ ] Handle case when local repo is behind remote. (Rebase? Merge? How?)
  - [ ] Add 'status' action which shows sync status
  - [ ] Commit on all modifications.
- UX
  - [ ] Add "summary" command to show tasks and prioritoes across contexts
  - [ ] Change delete from removal to archival in separate file.
  - [ ] Add `due:Friday` smart parsing
  - [ ] Add `due:"in two weeks"` smart parsing
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

- https://github.com/gammons/todolist Also in Go, similar idea with good query parsing
- Command line journaling: http://jrnl.sh/
