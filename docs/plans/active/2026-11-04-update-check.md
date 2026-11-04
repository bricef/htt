# Update check: tell users when a new version is out

**Date:** 2026-11-04
**Status:** In progress

## Motivation

htt is installed with `go install`, so users have no way to learn that
a newer version exists. A light check against GitHub, made when htt is
used interactively, closes that gap. It must be easy to turn off, never
get in the way, and be honest about the one network request it makes.

## Decisions captured

From the design dialogue (2026-11-04):

1. **A version is a GitHub Release.** The check asks for the latest
   published release (`/releases/latest`, which skips drafts and
   pre-releases) and compares its tag with the running binary's
   version by semver. Comparing against commits on `main` was
   rejected: every push would nag. The repository has no releases
   yet, so nothing is announced until the first one is published.

2. **The running version comes from Go's build info.** `go install
   …@vX.Y.Z` stamps `vX.Y.Z`; `@latest` without tags and local builds
   stamp a pseudo-version (`v0.0.0-<time>-<sha>`, `+dirty` when the
   tree had edits), which semver orders before any later release.
   A binary without a version (`(devel)`) doesn't check. `htt
   --version` prints it.

3. **Only GitHub.** The one request is
   `GET https://api.github.com/repos/bricef/htt/releases/latest` with
   `User-Agent: htt/<version>`. No token, no task data, nothing else.
   The URL is fixed at build time; no config or environment variable
   can change it.

4. **At most once a day.** The time of each attempt is written to
   `<user cache dir>/htt/update-check.json` (`~/.cache/htt/` on Linux)
   *before* the request goes out, so an offline or slow network can't
   cause a retry on every command.

5. **Failure is silent.** Offline, a DNS failure, a timeout (3s), a
   rate limit, an HTTP error, a malformed response, or a cache file
   that can't be read or written: none of these is an error for the
   command or prints anything. The TUI records them in its
   `--log-file` at warn level.

6. **Never in the way.** The request runs in the background while the
   command runs. Afterwards the CLI waits at most one more second
   for it, and only on the run that made the day's check.

7. **Only when someone is watching.** The CLI checks only when stderr
   is a terminal, so scripts, pipes and cron jobs never touch the
   network, and prints the notice to stderr after the command. The TUI
   shows it on its status line, and the full notice prints after the
   TUI exits. Shell completion (`__complete`, `completion`) and `help`
   never check. A notice appears only on the run that made the check,
   so at most once a day.

8. **Off switch:** `check_for_updates: false` in the config file
   (default `true`, written into new configs).

## Shape

- `internal/update`: the release URL, build-info version, the cached
  state, `Start` (load the cache, and if a check is due record the
  attempt and fetch in the background), `Check.Wait` / `Notice`.
  Imports nothing else from `internal/`.
- `internal/cli`: start the check once per process from the root
  command's `PersistentPreRun` when allowed; `htt interactive` passes a
  notice source to the TUI; `PrintUpdateNotice` for `main.go`;
  `RootCmd.Version`.
- `internal/tui`: `Options.UpdateNotice`; `Init` waits for it in the
  background and shows the notice on the status line.
- `internal/vars`, `cmd/htt/main.go`: the `check_for_updates` key and
  default; print the notice after `Execute`.

## Tests

- `update`: newer, same, older, pseudo-version, dirty and devel
  versions; a fresh cache makes no request; a stale cache makes one,
  recording the attempt first; the request's URL, User-Agent and lack
  of credentials; each failure (connection refused, timeout, 404,
  403, 500, bad JSON, unwritable cache) yields no notice and no
  error; the default URL is GitHub's.
- `cli`: when the check may run (enabled, command, terminal).
- `tui`: the notice reaches the status line.
- e2e: `htt --version`; commands without a terminal never open a
  connection, checked through a local proxy that counts them.
- Real terminal: the only host contacted is `api.github.com`; with a
  stand-in release server, the notice in the TUI and after exit.

## Out of scope

- Self-update, or any download.
- Pre-release channels.
- Publishing the first release (the user's call).
