# TODO

Holding pen for known issues and the wishlist. Wishlist items haven't
been triaged; some will graduate to GitHub Issues, some will be deleted
as obsolete, some will become plan documents under
`docs/plans/active/` when their time comes. Add new items here freely;
the README itself stays a getting-started doc.

Scoped work lives in [GitHub Issues](https://github.com/bricef/htt/issues).

## Known issues

Each confirmed against `main` on 2026-11-03.

- **`@end` isn't a sentinel.** After `htt log end`, `htt log status`
  reports "Currently working on: @end". Fixing it needs an open/closed
  model for entries (see the wishlist).
- **`:` in the TUI keymap does nothing yet** (command mode).
- **htt doesn't build for Windows** (confirmed 2026-11-17), so there's
  no Windows binary and `go install` fails there too: goterm v1.0.3
  uses `unix.EOPNOTSUPP` on every platform. goterm v1.0.4 fixes it
  (htt then builds and vets for windows/amd64 and windows/arm64);
  after the bump, add both to the platforms in `scripts/dist.sh`.

## Wishlist

- Tasks
  - [ ] Dedup: find duplicate and similar tasks (edit distance?)
  - [ ] Manage contexts: archive, delete, merge
  - [ ] Complete tasks across contexts (`htt do context/line`)
  - [ ] "Where is <>?": which context holds a task
  - [ ] Restore a deleted task from the archive (`htt todo restore`)
  - [ ] Query language (`htt todo show --filter` covers the common
        cases)
  - [ ] More `show` views: multi-key sort, negated filters, created-date
        ranges, saved views
- Time log
  - [ ] Open/closed entries, so `@end` (and a pause) mean something
  - [ ] `log@`: log an entry at a given time
  - [ ] Interval output
  - [ ] Review jrnl for inspiration
- Reports
  - [ ] Calendar modes (`--week`, `--month`)
  - [ ] Time per task
- Interactive mode
  - [ ] Fix help menu column balance
  - [ ] Command mode (`:`)
  - [ ] Sort, as `htt todo show --sort` does
  - [ ] Say why the TUI closed, after it exits
  - [ ] Themes: paint the whole screen background, a `--theme` flag or
        preview for trying themes, light/dark adaptive colours
- Editors
  - [ ] Parse `$EDITOR` like git does (`EDITOR="code --wait"` fails to
        start today) and honour `$VISUAL`
- REPL
  - [ ] Tab completion
  - [ ] Fixed-position prompt
- Code
  - [ ] Fork goparsec to fix messed up API choices (AST/Node
        distinction? simple querying)

## Production grade

- [ ] More packaging: a Homebrew tap, and macOS binaries signed and
      notarised (a binary downloaded with a browser is quarantined
      today)

## Future

- [ ] Create a GUI app layer (https://github.com/avelino/awesome-go#gui, https://github.com/murlokswarm/app)

## Done

- [x] A task typed with a date htt can't read warns as it's written
      (2026-11-18): `todo add`, `add-to`, `replace`, `edit` and
      `log add` name the line it went to on stderr, instead of leaving
      it to the next command that reads the file
- [x] The update notice suits every install (2026-11-18): it gives the
      release page to download from as well as `go install`, and the
      TUI's one-line form fits an 80-column status line
- [x] Release binaries (2026-11-17): every release gets archives for
      Linux and macOS on amd64 and arm64, a SHA256SUMS file and
      build-provenance attestations, built by GitHub Actions
      (`release-binaries.yml`, `scripts/dist.sh`); `just release` waits
      for them and checks them, and `just dist` builds them locally.
      Was "set up packaging for release"
- [x] Dates htt can't read warn instead of passing silently
      (2026-11-17): an impossible or unpadded date (`2026-13-45`,
      `2026-5-3`) and a `due:` that isn't a date still load as before,
      and the CLI names the file and line on stderr, once per line per
      run; the TUI uses its status line
- [x] `htt config dir` prints the config directory, not the tracker
      directory (2026-11-17)
- [x] `todo edit-done`, `todo edit-archive` and `log edit` return an
      error like `todo edit` on an unset `$EDITOR` or an editor
      failure, instead of exiting through `utils.Fatal` (2026-11-17)
- [x] `log start`, `log end` and `workon` print `Logging entry: …`
      again, like `log add` (2026-11-17)
- [x] REPL: a command's output stays on screen until the next command
      (2026-11-17)
- [x] REPL history is saved, in the config directory (2026-11-17)
- [x] `htt report` no longer lists deleted tasks under Added
      (2026-11-17)
- [x] Releases: `just release-check vX.Y.Z` and `just release vX.Y.Z
      notes.md` run the steps in `docs/releasing.md` (2026-11-17)
- [x] CI on GitHub Actions: tests, vet, lint, tidy modules and
      govulncheck on every push and weekly (2026-11-17)
- [x] Removed `internal/gcal.go` (2026-11-17), a standalone Google
      Calendar agenda printer copied in from elsewhere; with it went the
      Google API client, gRPC, OAuth and OpenTelemetry
- [x] Removed `htt sync` and its go-git backend (2026-11-17): sync
      will be a separate engine. Old config keys are ignored, and a
      `.git/` in the data directory is left alone
- [x] A rewrite never changes what a task means: when removing a
      priority or a leading annotation would leave `x`, `(B)` or a date
      at the front, the task gets today's date there first (2026-11-17)
- [x] Rewriting a task keeps every word where it was written:
      annotations stay in place and in order, and a repeated key keeps
      every occurrence (2026-11-12)

- [x] First release: v0.1.0 (2026-11-12); the process is in
      `docs/releasing.md`
- [x] Update check: at most once a day, GitHub only, silent on failure,
      `check_for_updates: false` to turn off; `htt --version`

- [x] Interactive: edit a task in place (`e`), with a live preview of
      the saved line, `due:friday` phrases and Tab-completed tags
- [x] Interactive: task lines coloured by part (priorities, the three
      kinds of tag, annotations, dates) in every theme, with search
      matches underlined and long rows cut to the screen
- [x] Interactive: the task list pages, and `/` searches with text and
      the `--filter` words; the cursor follows a task whose priority
      change re-sorts it
- [x] Interactive: colour themes (`tui.theme`: monokai, vscode,
      dracula, nord, solarized, gruvbox, …) and per-colour overrides
      (`tui.colors`)
- [x] Interactive: edit the context file in `$EDITOR` (`f`), without
      the TUI fighting the editor for the terminal
- [x] Interactive: diagnostics to `--log-file`, never the terminal
- [x] Show tasks in priority order (`htt todo show --sort priority`)
- [x] Interactive shell (`htt repl`); was "set up interactive cli
      with go-prompt"
- [x] Full-screen UI (`htt interactive`); was "set up a CLI GUI like
      todotxt-machine"
- [x] Interactive: new tasks get a created-on date (`Context.AddTask`
      stamps it)
- [x] Integration tests for mutating commands (`test/e2e`)
- [x] Archive deleted tasks (`archive.txt`, shown in `htt report`)
- [x] Interactive: Add new task
- [x] Interactive: Manage priorities
- [x] Interactive: autosort task list
- [x] Set up interactive mode with Bubbletea, bubbles and lipgloss
- [x] Add "summary" command to show tasks and priorities across contexts
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
