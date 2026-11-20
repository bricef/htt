# TODO

Holding pen for known issues and the wishlist. Wishlist items haven't
been triaged; some will graduate to GitHub Issues, some will be deleted
as obsolete, some will become plan documents under
`docs/plans/active/` when their time comes. Add new items here freely;
the README itself stays a getting-started doc.

Scoped work lives in [GitHub Issues](https://github.com/bricef/htt/issues).

## Known issues

Each confirmed against `main` on 2026-11-03.

- **`:` in the TUI keymap does nothing yet** (command mode).
- **Where the clocks go forward at midnight, a timesheet ends that
  night an hour early** (confirmed 2026-11-18, in `America/Santiago`
  on 2026-09-06): that midnight doesn't exist, and `time.Date` puts
  it an hour early, at 23:00 the day before. So `Timesheet` ends
  2026-09-05 at 23:00 (an entry at 22:00 ended at 23:30 counts 1h), in
  `htt report` and `htt gui` alike. Zones whose clocks change at 01:00
  or later, as in Europe and the US, aren't affected. (`htt report`
  also listed 2026-09-05 twice; since 2026-11-20 it reads days by
  date, as `htt gui` does, and lists it once.)

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
  - [ ] Review jrnl for inspiration
- Reports
  - [ ] Time per task
- Interactive mode
  - [ ] Fix help menu column balance
  - [ ] Command mode (`:`)
  - [ ] Sort, as `htt todo show --sort` does
  - [ ] Say why the TUI closed, after it exits
  - [ ] Themes: paint the whole screen background, a `--theme` flag or
        preview for trying themes, light/dark adaptive colours
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

- [ ] GUI: a native window around `htt gui`'s page (Wails v3 can wrap
      it); colours for categories in the config (a `color` key); push
      updates (SSE) rather than the page's refresh every minute; the
      config read again without a restart; task views and editing
      entries. Left out of the GUI plan

## Done

- [x] Calendar modes (2026-11-20): `htt report --week [DATE]` and
      `--month [MONTH]` report a week, Monday to Sunday, or a calendar
      month, with `htt gui`'s figures: the week's days and summary;
      the month's summary, weeks, average weekday and category table.
      The GUI and the report take every figure from the domain
      (`timesheet_figures.go`), and an e2e test checks the report
      against the GUI's API field by field. `--since`'s time section
      moved onto the same functions, its output unchanged but for the
      night the clocks go forward at midnight, now one day; see
      `docs/plans/complete/2026-11-20-report-periods.md`
- [x] The REPL opens the TUI (2026-11-20): `/interactive` and `/i`
      run the TUI on the REPL's terminal, which closes its readline
      first and opens a new one, on the same history, when the TUI
      quits; back at the prompt, the REPL shows the context the TUI
      left current. Without a terminal (stdin or stdout a pipe) it
      says the TUI needs one. `/repl` and `/gui` stay refused, aliases
      included. `/i` used to slip past the refusal of `/interactive`
- [x] The GUI: the time log in the browser (2026-11-19): `htt gui`
      serves a page from the binary on 127.0.0.1 and opens it: each
      day as a timeline coloured by category, the week's totals, and a
      month's report (a calendar, weeks, the average week, rolling
      category trends, halves). Go computes the figures (period
      aggregates in the domain), the page draws them with ECharts; no
      cgo, nothing fetched from the network. Was the wishlist's
      "Interval output" and the Future's "Create a GUI app layer";
      see `docs/plans/complete/2026-11-19-gui-time-log.md`
- [x] Windows (2026-11-18): htt builds for Windows with goterm
      v1.0.4, CI builds, vets and tests it there, and releases after
      v0.4.0 have Windows binaries, a zip holding `htt.exe` for amd64
      and arm64. Fixed on the way: the first run couldn't make its
      config directory there, the REPL read nothing from a pipe, and
      colours could print as escape codes (fatih/color v1.19.0 turns
      them on). Skipped on Windows: the time-log e2e tests that set
      htt's zone with TZ (Go ignores it there) and those whose editor
      is a shell script. Was the known issue "htt doesn't build for
      Windows"
- [x] Status flags a forgotten `log end` (2026-11-18): `htt log status`
      and `htt status` look at the last day logged before today, up to
      two weeks back, and when it has no `@end` add a line after the
      status with the `htt log end --at` that ends it; `log active`
      stays one line. Was the categories plan's decision 4, left
      unbuilt
- [x] htt picks and runs your editor as git does (2026-11-18):
      `$VISUAL`, unless `TERM` is `dumb` or (outside Windows) unset,
      then `$EDITOR`; a value with spaces or shell characters runs
      through `sh` with the file as its last argument, so
      `EDITOR="code --wait"` works, and `EDITOR=:` opens none. On
      Windows the value is split into words, double quotes grouping
      them. The CLI's edit commands and the TUI's `f` share one
      launcher (`internal/editor`)
- [x] Log a time-log entry at a given time (2026-11-18): `log start`,
      `log add`, `log end` and `workon` take `--at` (`HH:MM` today,
      `"yesterday HH:MM"` or `"YYYY-MM-DD HH:MM"`), which puts the
      entry into that day's log in time order and says when in the
      confirmation; a time later than now is refused. `htt report`'s
      flag on a day without an `@end` gives the `htt log end --at` that
      ends it. Was the wishlist's `log@`. ISO 8601 times too
      (`2026-11-17T17:30`, seconds and a zone optional), so a `ts:`
      copied from the log works
- [x] Time log categories, breaks and day ends (2026-11-18):
      `timelog.categories` sorts entries by tag (exact, first match
      wins) and `work: false` keeps a category such as breaks out of
      worked time; `@end` ends the day, the gap after it counting as
      nothing, and a day without one runs to 24:00 and is flagged.
      `htt report` gives each day's worked time and time per category,
      with totals, today's open entry counting until now; `log status`
      and `log active` say "Not working (day ended at HH:MM)" after
      `@end`. Was the known issue "`@end` isn't a sentinel" and the
      wishlist's "open/closed entries, so `@end` (and a pause) mean
      something"
- [x] `just release-check` (and `just release`) warns when GitHub has
      paused the weekly scan (2026-11-18), with the command that turns
      it back on; a warning only, and a failed lookup says so and
      carries on
- [x] Time logs are checked for dates htt can't read (2026-11-18):
      the `log` commands but `log edit`, `status`, `workon` and
      `htt report` warn about such entries on stderr as they do for
      task files, naming the log file and line, once per line per run;
      `ts:` never warns
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
- [x] REPL reads quotes as the shell does, so every `--at` form works
      there; apostrophes in words stay (2026-11-18)
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
