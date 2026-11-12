# TODO

Holding pen for known issues and the wishlist. Wishlist items haven't
been triaged; some will graduate to GitHub Issues, some will be deleted
as obsolete, some will become plan documents under
`docs/plans/active/` when their time comes. Add new items here freely;
the README itself stays a getting-started doc.

Scoped work lives in [GitHub Issues](https://github.com/bricef/htt/issues):
sync's problems and future are
[#12](https://github.com/bricef/htt/issues/12).

## Known issues

Each confirmed against `main` on 2026-11-03.

- **Sync** has two serious bugs and several limits, all in
  [#12](https://github.com/bricef/htt/issues/12): with no
  `backing_repo_url` it pushes into the git repository of the current
  directory, the default `ssh_key_file` is a public key, it never
  pulls, and the default `file_match_patterns` entry `.context`
  matches nothing.
- **`@end` isn't a sentinel.** After `htt log end`, `htt log status`
  reports "Currently working on: @end". Fixing it needs an open/closed
  model for entries (see the wishlist).
- **`log start`, `log end` and `workon` print nothing.** They stopped
  confirming their entry in the 2026-05-15 timelog refactor; `log add`
  still prints `Logging entry: …`.
- **REPL history isn't saved.** `replHistoryPath` builds the path from
  `config_path`, which is the config *file*, so creating the directory
  fails and history is switched off. It works within one session.
- **REPL output disappears.** Each cycle clears the screen straight
  after the command runs, so a command's own output (`Added: …`,
  search results, `/report`) is gone at once.
- **`htt report` counts deleted tasks as added**, under a context named
  `archive`: the Added section walks the archive like any other
  context.
- **`htt config dir` prints the tracker directory**, not the config
  directory its name suggests.
- **CLI edit commands exit through `utils.Fatal`**: `todo edit-done`,
  `todo edit-archive` and `log edit` end with a log-style message on
  an unset `$EDITOR` or an editor error, instead of returning an error
  like `todo edit` does.
- **Malformed dates load as text.** A line like `2026-13-45 task` is
  accepted rather than rejected.
- **`:` in the TUI keymap does nothing yet** (command mode).

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
  - [ ] Edit a single task (`f` edits the whole file)
  - [ ] Highlight priorities, tags, key:value pairs and dates
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
  - [ ] Remove `internal/gcal.go`, a dead Google Calendar experiment
        that alone pulls the Google API client, grpc and OpenTelemetry
        into `go.mod`
- Sync: see [#12](https://github.com/bricef/htt/issues/12)

## Production grade

- [ ] Automate releases (a `just release` recipe or goreleaser) around
      the manual steps in `docs/releasing.md`
- [ ] Set up packaging for release (see https://github.com/goreleaser/goreleaser)
- [ ] Set up CI

## Future

- [ ] Create a GUI app layer (https://github.com/avelino/awesome-go#gui, https://github.com/murlokswarm/app)

## Done

- [x] First release: v0.1.0 (2026-11-12); the process is in
      `docs/releasing.md`
- [x] Update check: at most once a day, GitHub only, silent on failure,
      `check_for_updates: false` to turn off; `htt --version`

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
