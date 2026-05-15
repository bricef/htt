# Hypothetical Tasks & Time Tracker (htt)

`htt` is a [todo.txt](http://todotxt.org/)-compatible command-line todo
list manager and time tracker. Tasks live in plain-text files; an
optional git-backed sync mode is included for cross-machine use.

## Installation

With a working Go toolchain:

```
go install github.com/bricef/htt/cmd/htt@latest
```

Make sure `$(go env GOBIN)` (or `$(go env GOPATH)/bin` if `GOBIN` is
unset) is on your `PATH`.

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

### Tip

Aliases keep the most-used subcommands close at hand:

```shell
alias t="htt t"      # htt todo
alias jnl="htt l"    # htt log
```

## Architecture

Layered around a domain package that owns the contracts and storage
implementations that satisfy them. CLI (cobra) and TUI (bubble tea)
sit at the top.

```
cli ─────────────► tui
 │                  │
 ▼                  ▼
storage ─────────► domain
```

The full tour — domain types, repository interfaces, storage impls,
how features flow through the layers, and how to add one — lives at
[`docs/architecture.md`](docs/architecture.md). Historical design
docs are under [`docs/plans/`](docs/plans/).

## Development

`justfile` is the source of truth for build / test / lint commands.
`just` with no arguments lists every recipe:

```shell
$ just
Available recipes:
    build        # Build the htt binary into ./bin/htt.
    check        # Lint with golangci-lint.
    clean        # Remove build artifacts and the in-repo Go cache.
    install      # Install the htt binary into $GOBIN / $GOPATH/bin.
    mod-download # Populate the module cache.
    test         # Run every test suite (e2e, TUI, internal packages).
    test-e2e     # Run just the e2e CLI harness.
    test-pkg pkg # Run one package, e.g. `just test-pkg internal/storage`.
    test-tui     # Run just the TUI snapshot harness.
    test-unit    # Run all internal/* tests.
    vet          # Static analysis across the module.
```

Quick dev loop for the TUI:

```shell
$ go run cmd/htt/main.go interactive --debug
# in another shell:
$ tail -f debug.log
```

## Wishlist

Rough lists of unscoped ideas (some will graduate to GitHub Issues)
live in [`TODO.md`](TODO.md).

## Prior art

- https://github.com/google/go-github — Github API client
- https://github.com/gammons/todolist — Also in Go, similar idea with good query parsing
- http://jrnl.sh/ — Command-line journaling
- https://github.com/VladimirMarkelov/ttdl — Terminal Todo List Manager in Rust
- https://github.com/hugokernel/todofi.sh — todotxt with [Rofi](https://github.com/davatorium/rofi)
- https://xwmx.github.io/nb/
- https://github.com/yuucu/todotui
- https://codeberg.org/pter/pter
- https://github.com/Fanteria/todotxt-tui
- http://todotxt.org/
- https://www.solidtime.io/ / https://github.com/solidtime-io/solidtime
