# GUI: the time log, seen

**Date:** 2026-11-19
**Status:** Complete (2026-11-19)

## Outcome

Built as planned, on `feat/gui`, in six commits: the period
aggregates in the domain, the server and its API, the page and its
assets, the `htt gui` command, a browser smoke test, and the docs.
The binary grows from 17,875,384 to 20,533,351 bytes (+2.7 MB), or
from 12,587,170 to 14,827,682 (+2.2 MB) as releases build it
(`-trimpath -ldflags='-s -w'`), linux/amd64, cgo off: 1.2 MB of
embedded assets, the rest the HTTP server, the API and the opener.

Choices made while building:

- **The domain's period aggregates** are in `timesheet_period.go`:
  `Totals` (worked; breaks, the rest of the time logged, uncategorised
  time included when the default category isn't work; by category;
  uncategorised; and `ToMidnight`, the last entry of each day without
  `@end`), `Period` with `WeekOf` and `MonthOf` (days held as dates at
  noon, so counted by date), `Weeks`, `Halves`, `Rolling`,
  `WeekdayAverages` and `Summarise`. `Timesheet` gained `Date`, the
  log's date at noon: where the clocks go forward at midnight its
  `Start` is on the day before, and a week or a month needs each day's
  weekday and date.
- **`future` is the GUI's, not the domain's.** A `DayStatus` says how
  a log stands when read at now; a day after today has no log to read,
  so the server doesn't load one (a hand-written log for tomorrow is
  ignored), and the day has no time.
- **This week's and this month's figures are over the days so far**,
  today included: the summary, the averages (the average week is
  worked ÷ days so far × 7), the weekday averages, the rolling totals
  and the halves. Decision 5 says "worked ÷ days in the month × 7",
  which for a past month is the same; for this month, dividing by its
  days to come would understate every average. The weeks chart shows
  all the month's weeks, those to come empty. Brice agreed
  (2026-11-19): "it's meaningless to average over data we don't have."
- **The timeline opens** on the prototype's rule: from the whole hour
  at least half an hour before the first entry to the whole hour at
  least half an hour after the last end (24:00 when a day has no
  `@end`, as decision 4 says); the whole day when nothing is logged.
  A *Whole day* button zooms out and back, besides the slider.
- **The API's categories** are the config's, in its order, those with
  tags only (a default without tags, such as the built-in `working`,
  can never have time of its own), then `uncategorised`, with
  `countedAs` naming the default. The colour index counts the work
  categories and the others separately, as the prototype coloured
  `htt` teal with `break` first in the list.
- **Colours**: the prototype's teal, amber and violet, then five more
  work colours, ordered so neighbours in a stack stay apart for
  colour-blind readers (checked with a palette validator, light and
  dark); four muted tones for categories that aren't work;
  uncategorised grey. The approved teal and dark tones sit just
  outside that validator's chroma and lightness bands; they're kept as
  approved.
- **Each day of the month has its intervals**, as decision 9 says,
  though the month view draws none: 58 KB for a full month of
  realistic logs, refetched every minute only for this month.
- **A config htt can't use**: the API answers 500 with its error and
  "fix it, then restart htt gui"; the page shows that instead of the
  views, and `htt gui` warns once on stderr. It draws no time without
  categories, as `htt report` stops rather than report it.
- **The browser opens** (replaced after review, below) through
  `github.com/cli/browser` v1.3.0, the GitHub CLI's opener, rather
  than a hand-rolled one: it also covers WSL (`wslview`) and uses
  ShellExecute on Windows, and its one dependency, `golang.org/x/sys`,
  was already in htt's. Its output is dropped; a failure says so on
  stderr.
- **Fonts** from IBM's npm packages (`@ibm/plex-sans` 2.0.0,
  `@ibm/plex-mono` 2.5.0, static woff2, Latin1), not Google Fonts,
  whose Plex Sans is now one variable font: pinned versions, checked
  against the registry's integrity. Same design as the prototype's.
  ECharts' files include `LICENSE-d3`, which its LICENSE names, and
  ZRender's BSD licence, since `echarts.min.js` bundles ZRender and
  its LICENSE doesn't say so. `.gitignore`'s `vendor`, meant for the
  module's vendor directory, ignored `web/vendor` too; it's now
  `/vendor`.
- **The guards**, in order: Host (403), token (404, constant-time;
  dropped after review, below), method (405). The API's two paths are
  matched exactly and anything else must be a regular file of `web/`,
  so `..` and empty path elements reach nothing without cleaning. A
  Content-Security-Policy keeps the page to its own files; styles may
  be inline (the tooltips' HTML has style attributes), scripts may
  not, and ECharts needs no `eval` (its one `new Function` parses
  GeoJSON strings, unused).
- **The address** comes from the listener itself, so it says what was
  bound; the browser test and the e2e tests read it from stdout.
- **chromedp v0.22.1**, the newest release at least two weeks old, on
  its new generic API (v0.20 on); test-only. The browser test finds
  Chrome in `$HTT_TEST_CHROME`, on `PATH`, or where macOS and Windows
  install it. On the dev box that's a script that sets
  `LD_LIBRARY_PATH` and execs Playwright's `chrome-headless-shell`.

Tests, as planned: tables and properties for the domain (150 generated
months in six zones, months starting on every weekday); golden JSON
for a week and a month and the guards for the server; e2e with the real
binary (the interrupt skipped on Windows, where Go can't send one to
another process); and the browser smoke test. Breaking each rule in
turn, 63 mutations (20 in the aggregates, 28 in the server and API, 7
in the command, 8 in the page), failed a test every time; two guards
that no test could break were redundant and were removed (path
cleaning, and an `fs.ValidPath` check that `fs.Stat` already makes).

Screenshots of both views, light and dark, 1180 and 400 px wide, were
checked by eye; at phone width the weeks chart shortens its labels.

Left open:

- **The config is read when `htt gui` starts**; a change needs a
  restart, which the page and the help say. Reading it per request
  would mean the GUI reading viper, or the CLI handing it a func.
- **Release archives don't carry the third-party licences as files.**
  They're in the binary and served by `htt gui`, but Apache-2.0
  asks redistributors to give a copy of the licence and NOTICE;
  `scripts/dist.sh` could add them (htt's Go dependencies' licences
  aren't shipped either). Done 2026-11-20 (merge 0b87ecc), in every
  archive from v0.6.0: `third-party/` holds the licences, notices and
  sources README from `internal/gui/web/vendor`. The Go dependencies'
  licences weren't part of it.
- **`/i` runs the TUI inside the REPL**: the REPL refuses
  `interactive` but not its alias `i`. Found while adding `gui`; not
  fixed here. Done 2026-11-20 (merge d3b0955): the REPL opens the TUI
  on purpose, with `/interactive` or `/i`, and hands the terminal
  back when it quits. It refuses `repl` and `gui` by the command
  itself, so an alias can't slip past.
- **`htt report` could share the aggregates** (the wishlist's calendar
  modes, `--week` and `--month`). Done 2026-11-20 (merge 2f14c28):
  `htt report --week` and `--month` print the GUI's figures from the
  same domain functions; see
  `docs/plans/complete/2026-11-20-report-periods.md`.
- **The browser test hasn't run on CI yet.** CI's Linux and Windows
  runners have Chrome, so it will run there. Done the same day: it ran
  on `feat/gui`'s CI, Linux and Windows, before the merge (PR #14),
  and failed there on Chrome asking for `/favicon.ico`, which the
  headless shell used locally never does (fixed in bce58eb).
- Out of scope as planned: a native window, editing, task views,
  colours in the config, push updates.

After review (an independent review of `feat/gui`, fixed on the same
branch, each fix with a test seen failing first and a mutation check):

- **The token was on the opener's command line**, for any local user
  to read in `ps`. htt opened a private file instead (0600, in a new
  0700 temporary directory, removed when it stopped) that redirected
  to the address, as Jupyter does, until the token itself went (below).
  `github.com/cli/browser` couldn't do that or the next fix, so it was
  replaced by htt's own opener (`open`; `xdg-open`, `x-www-browser`,
  `www-browser`, `wslview`, then `explorer.exe` on WSL; ShellExecute
  on Windows), which stays; the dependency is gone, and
  `golang.org/x/sys` is now a direct one.
- **Ctrl-C reached the opener**, and with it a browser it had started:
  the opener now runs in a process group of its own, and one stopped
  by a signal isn't reported as a failure.
- **Stopping** waited five seconds with a connection open and exited 1;
  now requests under way get a second, a second Ctrl-C cuts it short,
  the rest are closed, and it exits 0.
- **Dates in the API** are read as calendar dates: at local midnight,
  where that midnight doesn't exist (America/Asuncion), October was
  September.
- **The page**: the weeks chart's axis reaches the average week when
  it's above every bar (it vanished); categories named after
  `Object.prototype`'s members no longer show `NaNm`; a pinned entry
  goes when the period changes; an entry in the hour that repeats is
  drawn for its true length (the API's new `drawnEndMinute`), its
  tooltip keeping its clock times.
- **Smaller**: `--port 80` accepts the bare Host browsers send; the
  CSP test checks the whole policy; `TestListenAndServe` holds a
  request under way.
- **Open** then: a sandboxed browser that can't read the temporary
  directory (Snap's Firefox, which has its own `/tmp`) would say it
  can't find the file. Gone with the token, below.

Decided after review (Brice, 2026-11-19):

- **No token.** The address is plain, `http://127.0.0.1:<port>/`: the
  page at `/`, the API at `/api/week` and `/api/month`, the assets at
  their paths. The token stopped one threat only: other accounts on the
  same computer reading the time log while `htt gui` runs, as
  127.0.0.1 is open to every local user where htt's data directory is
  0700. Everything else is covered without it: a page elsewhere can't
  read the answers (same-origin policy; the server sends no CORS
  headers); DNS rebinding is stopped by the Host check; there's no
  CSRF to stop, as the GUI only reads; no other computer reaches
  127.0.0.1; and a process of the same user could read the files
  anyway. Go's own local read-only UIs (`go tool pprof -http`,
  `go tool trace`) make the same call. Keeping it cost the redirect
  file in the temporary directory, which snap browsers (Ubuntu's
  Firefox) can't open; a more complex opener; and an address that
  changed every run. Gone with it: the redirect file, its directory
  and their cleanup; `/<token>`'s redirect; `crypto/rand` and
  `crypto/subtle` in the server. The opener is still htt's own, with
  its process group on Unix and ShellExecute on Windows, and is given
  the address; `explorer.exe` on WSL takes it as it is, without
  `wslpath`. GETTING-STARTED says plainly that other accounts can see
  the page while it runs, and that `--port` makes the address one to
  bookmark. If the GUI ever writes, protect the writes then with
  Origin / Sec-Fetch-Site checks, not a token in the address.
- **Origin and Sec-Fetch-Site checks now**, for reads too (Brice asked
  for them the same day, once the token was gone). Every request,
  after the Host check: an `Origin`, when there is one, must be the
  GUI's own, `http://127.0.0.1:<port>` or `http://localhost:<port>`
  (either without the port on port 80, as the Host check allows);
  anything else, `null` included, is a 403. By `Sec-Fetch-Site`
  (Fetch Metadata resource isolation): absent (curl, an old browser),
  `same-origin`, or `none` (an address typed, a bookmark, htt's
  opener) passes; `same-site`, which a page on another port of
  localhost is, and `cross-site` pass only as a plain top-level
  navigation, a GET with `Sec-Fetch-Mode: navigate` and
  `Sec-Fetch-Dest: document`, so a link to the GUI still opens it; any
  other request (a fetch, an image, a script, a style, a frame, an
  object) is a 403, "htt gui answers its own page only", with the
  usual headers. Why: the same-origin policy keeps another site's page
  from reading the answers, but not from having the browser make the
  requests, loading the API or the page blind as an image, a script
  or a frame and learning from what happens (an error, the time it
  takes: a cross-site leak). Refusing them costs the GUI's own page
  nothing, as its requests are all same-origin. Not added:
  `http.CrossOriginProtection`, which acts on unsafe methods only,
  and the server refuses those with a 405; it's the tool to use if the
  GUI ever writes. Tested with a table of requests as browsers send
  them, seen failing first; breaking each rule in turn (20 mutations)
  failed it every time, and refusing `same-origin` failed the browser
  tests too.

## Motivation

The time log now knows categories, breaks and day ends (see
`docs/plans/complete/2026-11-18-timelog-categories.md`), but the only
way to see it is `htt report`'s text. Brice wants a GUI whose first job
is visualising the time log: each day as a timeline coloured by
category, the week's totals, and a month report with weekly averages
and category trends.

A survey of Go GUI options (2026-11-19) found that charts are easy in a
web view and hard natively: no maintained Go chart library draws
floating bars, while ECharts' "Profile" pattern is the day timeline
almost as is. Every native toolkit or webview window needs cgo on
macOS or Linux, which would end htt's single static binary and its
cross-compiled releases. So the GUI is a local web page served by htt
itself; a native window (Wails v3) can wrap the same page later.

A throwaway prototype (dummy data, ECharts 6.1) settled the look: the
style, the palette, a timeline that opens on the hours logged, breaks
in the totals, and the month views.

## Decisions

1. **`htt gui`** serves the GUI from the htt binary: no cgo, no second
   binary, release pipeline unchanged. It prints the address, opens it
   in the default browser (`--no-open` doesn't), and serves until
   Ctrl-C. `--port` picks the port; the default is a free one.
2. **Local.** It listens on 127.0.0.1 only, at a plain address,
   `http://127.0.0.1:<port>/`. A request whose `Host` isn't
   `127.0.0.1:<port>` or `localhost:<port>` is refused (DNS
   rebinding). It only reads: GET and HEAD only, nothing writes.
   (Amended after review: every path started with a random token,
   128 bits, new each run, and anything else was a 404; Brice dropped
   it, as other accounts on the computer are all it kept out, and
   added Origin and Sec-Fetch-Site checks: only the GUI's own page, or
   a person opening it, gets an answer. See "Decided after review" in
   the Outcome.)
3. **Go computes, the page draws.** The days, weeks, weekday averages,
   rolling trends and half-month comparisons are computed in Go, in the
   domain (pure functions over timesheets), where htt's tests can hold
   them to the same bar as the rest, and `htt report` can share them
   later. The page fetches JSON and only lays it out.
4. **Week view**: Monday to Sunday, the week of a given date (today by
   default), with previous / next / this week.
   - A timeline, a row per day: each entry from its start to the next,
     coloured by category; a day without `@end` drawn to midnight,
     faded and dashed, and labelled; today's last entry ending at a
     "now" mark; an empty day labelled; days after today labelled as
     not yet. It opens on the hours logged (from the hour before the
     first entry to the hour after the last end, a day without `@end`
     reaching 24:00) and zooms out to the whole day. Hover shows the
     entry; click pins it below the chart.
   - Week totals: stacked bars per day by category, each labelled with
     worked time and breaks (time in categories that aren't work), and
     a table of the week's totals per category.
5. **Month view**: a calendar month (this month by default), with
   previous / next / this month.
   - Summary: worked, average week (worked ÷ days in the month × 7),
     average working day (worked ÷ days with worked time), breaks,
     uncategorised share of worked, days without `@end`.
   - A calendar of worked time per day, a day without `@end` outlined.
   - Weeks: stacked bars per Monday-to-Sunday week, cut at the month's
     edges (labelled with their day counts), and the average week as a
     line.
   - Average week: per weekday, the average over that weekday's days in
     the month, stacked by category.
   - Category trends: per category, the rolling total of the 7 days up
     to each date (reaching into the previous month for the first
     days), in hours per week.
   - A table per category: the month's total, share of worked, hours per
     week in days 1–15 and 16–end, and the change.
6. **Categories and colours** come from `timelog.categories`, as the
   report's do. Colours are assigned by position from a fixed palette,
   one set for light and one for dark; categories that aren't work get
   muted tones; uncategorised is grey. (A `color` key in the config can
   come later.) A config htt can't use shows its error in the page,
   naming the key, as the report does.
7. **The page** is plain HTML, CSS and JavaScript, no build step,
   embedded with `go:embed`: ECharts 6.1.0 (`echarts.min.js`,
   Apache-2.0, with its licence and notice) and IBM Plex Sans and Mono
   (woff2, latin subset, OFL, with the licence), so nothing is fetched
   from the network at runtime. It follows the system's light or dark
   setting. The prototype's palette and type are the starting point.
8. **Fresh data**: the page refetches when it becomes visible again,
   and every 60 seconds while the period shown includes today.
9. **The JSON contract** (the API is internal to htt, but versioned in
   one place): durations in seconds; times as RFC 3339 with the local
   offset, plus minutes since local midnight on the wall clock for
   drawing (so a day the clocks change still draws by its clock, while
   durations stay true). Each day: date, status (`empty`, `ended`,
   `inProgress`, `unended`, `future`), worked, breaks, time per
   category, and intervals (entry text without its `ts:`, category,
   uncategorised, work, start, end, seconds, and `open`: `now`,
   `midnight` or none). Every response also carries the categories (in
   config order, with `work` and a colour index) and now.
10. **Not in the REPL**: `gui` blocks until Ctrl-C, so the REPL refuses
    it, as it refuses `interactive`.

## Shape

- `internal/domain`: period aggregation over timesheets (a week, a
  month, weeks within a period, weekday averages, rolling 7-day
  totals, halves). Pure; no I/O.
- `internal/gui` (new): the server (listener, Host check, shutdown;
  a token too, until review), handlers for the week, the month and
  the page's assets, JSON view models, and the embedded assets. It
  imports `domain`; the CLI hands it a timelog repository, a
  categoriser and a clock, so it imports neither `cli` nor
  `storage`. The architecture test gains its rules: `gui` doesn't
  import `cli` or `tui`; `cli` may import `gui`.
- `internal/cli`: the `gui` command, its flags, opening the browser,
  Ctrl-C.

## Testing

- **Domain**: table tests for each aggregate, including partial weeks,
  months starting on each weekday, empty and unended days, days the
  clocks change, and the rolling window across a month boundary.
  Property tests over generated months: the days sum to the month, the
  weeks partition it, each rolling total equals a direct sum of its
  7 days, averages times counts give totals. Mutation checks.
- **`internal/gui`**: handler tests with an in-memory repository and a
  fixed clock: the JSON for a known week and month (golden), the Host
  check (and the token's, until review), methods, bad dates, a bad
  config.
- **e2e**: the real binary with `htt gui --no-open --port 0`: read the
  printed address, fetch the week and month, check a wrong token is a
  404 (until review dropped the token), then stop it with an interrupt
  and see it exit cleanly.
- **Browser smoke test**: headless Chrome (chromedp) loads the page from
  the real binary and checks it draws both views without a JavaScript
  error; skipped where no Chrome is installed. CI's runners have one.
- **By eye**: screenshots of both views, light and dark, desktop and
  phone width, in the report back.

## Out of scope

- A native window (Wails v3 can wrap the same page later), editing
  entries, task views, colours in the config, push updates (SSE).
