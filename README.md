# Hypothetical Tasks & Time Tracker (htt)

`htt` is a [todo.txt](http://todotxt.org/)-compatible command-line todo
list manager and time tracker. Tasks live in plain-text files you can
also edit by hand.

- **Todo lists** in todo.txt format, split into contexts (`work`,
  `home`, …), with priorities, natural-language due dates
  (`--due "next Friday"`), search, and sorted or filtered views.
- **A time log**: a timestamped record of what you worked on each day.
- **Reports**: what you completed, added and deleted, and how long you
  worked, over any period.
- **Three ways in**: one-shot commands, a todo-mode shell (`htt repl`)
  and a full-screen TUI (`htt interactive`) with search, task lines
  coloured by priority, tag and date, colour themes (Monokai, VS Code,
  Dracula, Nord, Solarized, Gruvbox) and per-colour config.
- **Git sync** (`htt sync`): manual and push-only for now. Read
  [its setup and limits](GETTING-STARTED.md#optional-git-sync) before
  using it; [#12](https://github.com/bricef/htt/issues/12) tracks
  its problems.

New to `htt`? [`GETTING-STARTED.md`](GETTING-STARTED.md) is a
fifteen-minute walkthrough of the realistic workflow — add a task,
prioritise it, complete it, organise with contexts, log time against
it — and the reference for every command.

## Installation

With a working Go toolchain:

```
go install github.com/bricef/htt/cmd/htt@latest
```

Make sure `$(go env GOBIN)` (or `$(go env GOPATH)/bin` if `GOBIN` is
unset) is on your `PATH`.

htt needs Go 1.26.11 or newer, because earlier releases have
standard-library vulnerabilities its network code can reach. With an
older Go, `go install` downloads 1.26.11 for the build by itself,
unless you've set `GOTOOLCHAIN=local`.

Releases are listed on the
[releases page](https://github.com/bricef/htt/releases);
`htt --version` tells you which one you have.

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
  interactive Enter interactive mode
  log         Manage the activity log.
  repl        Interactive todo-mode shell.
  report      Summarise activity in a time period.
  status      Show the status of the tasklist and time log.
  sync        Commit the data directory and push it to the backing git repo.
  todo        Manage todo lists.
  workon      Log that work has begun on numbered item in the current context.

Flags:
  -h, --help       help for htt
      --no-color   Disable color output for coloured commands

Use "htt [command] --help" for more information about a command.
```

### Tips

Aliases keep the most-used subcommands close at hand:

```shell
alias t="htt t"      # htt todo
alias jnl="htt l"    # htt log
```

`htt completion --help` explains how to install tab completion for
bash, zsh, fish or PowerShell. `htt --version` prints the version you
have.

## Online activity

htt works offline. It reaches the network in two cases only, and
never sends your tasks or time logs anywhere you haven't configured.

### Update check

htt checks whether a newer release exists, so you hear about fixes.

- **What it contacts:** one URL,
  `https://api.github.com/repos/bricef/htt/releases/latest`, which
  answers with the newest published htt release. Nothing else is
  contacted, and a redirect to another host is refused.
- **What it sends:** a plain `GET` with a `User-Agent: htt/<your
  version>` header. No account, token, task data or settings. Like any
  request, it shows GitHub your IP address (see
  [GitHub's privacy statement](https://docs.github.com/site-policy/privacy-policies/github-general-privacy-statement)).
- **When:** at most once a day, and only when you're there to see the
  answer: a command run at a terminal, or the TUI. Scripts, pipes, cron
  jobs, shell tab completion and `help` never check, and neither do
  builds without a version (`htt --version` says "unknown").
- **What you see:** if the release is newer than yours, a notice after
  the command (on stderr) or on the TUI's status line, at most once a
  day. htt never downloads or installs anything itself.
- **If it fails** (offline, a timeout, a rate limit, any error):
  nothing. The command runs exactly as it would have, and nothing is
  printed; the TUI's `--log-file` records why. The request runs in the
  background, and a command waits at most a second for it, only on the
  day's checking run.
- **What it stores:** `update-check.json` in your user cache directory
  (`~/.cache/htt/` on Linux): when it last checked, and the latest
  release it saw.
- **Turning it off:** set `check_for_updates: false` in your config
  file (`~/.config/htt/config.yaml`).

### Sync

`htt sync` pushes your data directory to the git remote you configure
(`backing_repo_url`), over SSH, and only when you run it. See
[its section in GETTING-STARTED](GETTING-STARTED.md#optional-git-sync).

There is nothing else: no telemetry, no analytics, no other servers.
(Installing with `go install` downloads htt through the Go module
proxy; that's the Go toolchain, before htt ever runs.)

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

Quick dev loop for the TUI. It never writes to the terminal outside
its own screen; `--log-file` appends its diagnostics to a file, at
debug level unless `--log-level` (`debug`, `info`, `warn`, `error`)
says otherwise:

```shell
$ go run cmd/htt/main.go interactive --log-file debug.log
# in another shell:
$ tail -f debug.log
```

## Wishlist and known issues

[`TODO.md`](TODO.md) holds the wishlist of unscoped ideas and a list of
known issues. Work that's been scoped lives in
[GitHub Issues](https://github.com/bricef/htt/issues), starting with
[#12](https://github.com/bricef/htt/issues/12) for sync.

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
