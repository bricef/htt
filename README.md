# Hypothetical Tasks & Time Tracker (htt)

## Usage

```shell
$ htt help
Hypothetical tracker is a todo list manager and time tracker

Usage:
  ht [flags]
  ht [command]

Available Commands:
  add         Add an item to the default task list
  help        Help about any command
  show        Show the default tasklist.
  sync        Sync the data to the backend manually

Flags:
  -h, --help   help for htt

Use "ht [command] --help" for more information about a command.
```

## Todo

- [ ] Check if repo exists. If it doesn't clone it first.
- [ ] Handle case when local repo is behind remote. (Rebase? Merge? How?)
- [ ] Set up interactive cli