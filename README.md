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
  add         Add an item to the default task list
  context     Change the context for tasks
  delete      Delete the item specified
  edit        Edit the item specified using $EDITOR
  help        Help about any command
  replace     Replace an item with a new entry
  show        Show the default tasklist.
  sync        Sync the data to the backend manually

Flags:
  -h, --help   help for htt

Use "htt [command] --help" for more information about a command.
```

## Todo

- [ ] Check if repo exists. If it doesn't clone it first
- [ ] Show active and other contexts on `show`
- [ ] Delegate full editing to other program (todotxt-machine, say)
- [ ] Handle case when local repo is behind remote. (Rebase? Merge? How?)
- [ ] Set up interactive cli
- [ ] Set up packaging for release. (see https://github.com/goreleaser/goreleaser)