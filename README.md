# Hypothetical Tasks & Time Tracker (htt)

[![CI](https://github.com/bricef/htt/actions/workflows/ci.yml/badge.svg)](https://github.com/bricef/htt/actions/workflows/ci.yml)

`htt` is a [todo.txt](http://todotxt.org/)-compatible command-line todo
list manager and time tracker. Tasks live in plain-text files you can
also edit by hand.

- **Todo lists** in todo.txt format, split into contexts (`work`,
  `home`, …), with priorities, natural-language due dates
  (`--due "next Friday"`), search, and sorted or filtered views.
- **A time log**: a timestamped record of what you worked on each day,
  with breaks, day ends and categories by tag, and entries logged after
  the fact (`htt log end --at "yesterday 17:30"`).
- **Reports**: what you completed, added and deleted, and how long you
  worked, per category, over any period.
- **Three ways in**: one-shot commands, a todo-mode shell (`htt repl`)
  and a full-screen TUI (`htt interactive`) with search, in-place
  editing with a live preview and tag completion, task lines coloured
  by priority, tag and date, colour themes (Monokai, VS Code, Dracula,
  Nord, Solarized, Gruvbox) and per-colour config.

New to `htt`? [`GETTING-STARTED.md`](GETTING-STARTED.md) is a
fifteen-minute walkthrough of the realistic workflow — add a task,
prioritise it, complete it, organise with contexts, log time against
it — and the reference for every command.

## Installation

### Download a binary

Every [release](https://github.com/bricef/htt/releases) from v0.3.0
on has a ready-to-run `htt` for Linux and macOS, and every release
after v0.4.0 has one for Windows, so you don't need Go. Pick the
archive for your machine (`uname -sm` tells you which you have):

| Machine | `uname -sm` | Archive |
| --- | --- | --- |
| Linux on Intel or AMD | `Linux x86_64` | `htt_vX.Y.Z_linux_amd64.tar.gz` |
| Linux on 64-bit ARM | `Linux aarch64` | `htt_vX.Y.Z_linux_arm64.tar.gz` |
| Mac with Apple silicon | `Darwin arm64` | `htt_vX.Y.Z_darwin_arm64.tar.gz` |
| Mac with an Intel processor | `Darwin x86_64` | `htt_vX.Y.Z_darwin_amd64.tar.gz` |

On Windows, pick `htt_vX.Y.Z_windows_amd64.zip` for an Intel or AMD
processor, or `htt_vX.Y.Z_windows_arm64.zip` for an ARM one. Settings
› System › About says which you have, under "System type".

Download it and the checksums file, `htt_vX.Y.Z_SHA256SUMS`, from the
release page, or with [gh](https://cli.github.com/) (v0.3.0 on an
Apple silicon Mac, say):

```shell
gh release download v0.3.0 --repo bricef/htt \
  --pattern htt_v0.3.0_darwin_arm64.tar.gz --pattern htt_v0.3.0_SHA256SUMS
```

Check the archive against the checksums file. It should say `OK`:

```shell
grep htt_v0.3.0_darwin_arm64.tar.gz htt_v0.3.0_SHA256SUMS | shasum -a 256 -c   # macOS
grep htt_v0.3.0_linux_amd64.tar.gz htt_v0.3.0_SHA256SUMS | sha256sum -c        # Linux
```

On Windows, `Get-FileHash` in PowerShell prints the zip's SHA-256. It
should match the zip's line in the checksums file (in capitals):

```powershell
Get-FileHash htt_vX.Y.Z_windows_amd64.zip
```

Each archive also has a signed build-provenance attestation, which
shows it was built by htt's release workflow from htt's repository.
gh checks it:

```shell
gh attestation verify htt_v0.3.0_darwin_arm64.tar.gz --repo bricef/htt
```

Then unpack `htt` and put it somewhere on your `PATH`:

```shell
tar -xzf htt_v0.3.0_darwin_arm64.tar.gz htt
mv htt ~/.local/bin/
htt --version
```

On Windows, unpack the zip and put `htt.exe` in a folder on your
`PATH`. In PowerShell, to unpack it into `%LOCALAPPDATA%\Programs\htt`
and add that folder to your `PATH`:

```powershell
Expand-Archive htt_vX.Y.Z_windows_amd64.zip "$env:LOCALAPPDATA\Programs\htt"
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
[Environment]::SetEnvironmentVariable("Path", "$userPath;$env:LOCALAPPDATA\Programs\htt", "User")
```

Terminals opened after that have the new `PATH`. In a new one,
`htt --version` says which version you have.

On macOS, a file downloaded with a web browser is quarantined, and
macOS won't run a quarantined program that isn't signed by an Apple
developer, which htt isn't: it says Apple can't verify that "htt" is
free of malware. Clear the quarantine flag to run it:

```shell
xattr -d com.apple.quarantine ~/.local/bin/htt
```

(Downloads made with `gh` or `curl` aren't quarantined.)

To update, download the new release the same way; the update notice
links to it. On Windows, `Expand-Archive -Force` replaces the old
`htt.exe`.

### With Go

With a working Go toolchain:

```
go install github.com/bricef/htt/cmd/htt@latest
```

Make sure `$(go env GOBIN)` (or `$(go env GOPATH)/bin` if `GOBIN` is
unset) is on your `PATH`. On Windows, this needs a release after
v0.4.0: v0.4.0 and earlier don't build there.

htt needs Go 1.26.11 or newer, because earlier releases have
standard-library vulnerabilities its network code can reach. With an
older Go, `go install` downloads 1.26.11 for the build by itself,
unless you've set `GOTOOLCHAIN=local`.

Releases are listed on the
[releases page](https://github.com/bricef/htt/releases);
`htt --version` tells you which one you have.

### From source

htt takes its version from git: `go install …@vX.Y.Z`, or a build in a
clone checked out at a release tag, knows it. A build from a source
archive (the `.tar.gz` GitHub offers for each release, as a package
manager uses) has no git information, so pass the version in, or
`htt --version` says "unknown" and the update check stays off:

```
go build -ldflags "-X github.com/bricef/htt/internal/update.version=v0.3.1" -o htt ./cmd/htt
```

The `v` is optional (`version=0.3.1` works too), and a version given
this way wins over anything git says.

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

htt works offline. It reaches the network for one thing only, the
update check below, and never sends your tasks or time logs anywhere.

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
  day. It gives both ways to update, since htt can't tell how you
  installed it: the release page to download it from, and the
  `go install` command:

  ```
  A new version of htt is available: v0.4.0 (you have v0.3.0).
    Download it:  https://github.com/bricef/htt/releases/tag/v0.4.0
    Or with Go:   go install github.com/bricef/htt/cmd/htt@latest
  Turn this check off with check_for_updates: false in your config file.
  ```

  htt never downloads or installs anything itself.
- **If it fails** (offline, a timeout, a rate limit, any error):
  nothing. The command runs exactly as it would have, and nothing is
  printed; the TUI's `--log-file` records why. The request runs in the
  background, and a command waits at most a second for it, only on the
  day's checking run.
- **What it stores:** `update-check.json` in your user cache directory
  (`~/.cache/htt/` on Linux, `%LOCALAPPDATA%\htt\` on Windows): when
  it last checked, and the latest release it saw.
- **Turning it off:** set `check_for_updates: false` in your config
  file (`~/.config/htt/config.yaml`).

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
    build                    # Build the htt binary into ./bin/htt.
    check                    # Lint with golangci-lint.
    clean                    # Remove build artifacts and the in-repo Go cache.
    default                  # Default: list recipes.
    dist version             # Build the release archives for tag VERSION into dist/, as releases do.
    install                  # Install the htt binary into $GOBIN / $GOPATH/bin.
    mod-download             # Populate the module cache.
    release version notes    # Release main as VERSION with NOTES: check, tag, publish, verify.
    release-binaries version # Rebuild the binaries of the published release VERSION, and check them.
    release-check version    # Check main is ready to release as VERSION (vX.Y.Z); changes nothing.
    test                     # Run every test suite (e2e, TUI, internal packages).
    test-e2e                 # Run just the e2e CLI harness.
    test-pkg pkg             # Run one package, e.g. `just test-pkg internal/storage`.
    test-tui                 # Run just the TUI snapshot harness.
    test-unit                # Run all internal/* tests (domain, storage, usecase, todo, ...).
    vet                      # Static analysis across the module.
```

Releases are made with `just release`; see
[`docs/releasing.md`](docs/releasing.md). Publishing one runs
[`release-binaries.yml`](.github/workflows/release-binaries.yml),
which builds the binaries with `scripts/dist.sh` and attaches them to
the release; `just dist vX.Y.Z` builds the same archives locally.

CI ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)) runs the
same recipes on every push to any branch: `go mod tidy -diff`,
`just test`, `just vet` and `just check` (golangci-lint v2.15.0),
plus govulncheck, and it builds, vets and tests htt on Windows. A
separate workflow,
[`weekly.yml`](.github/workflows/weekly.yml), runs govulncheck on
Mondays, so a vulnerability published while htt is quiet still shows
up. GitHub pauses a scheduled workflow after 60 days without activity
and doesn't resume it by itself; turn it back on with
`gh workflow enable weekly.yml` (`just release-check` warns when it's
off). The workflows' actions are pinned to
commits, and Dependabot updates them monthly.

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
[GitHub Issues](https://github.com/bricef/htt/issues).

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

## License

htt is released under the [MIT License](LICENSE).
