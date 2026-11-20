# Report: a week or a month, with the GUI's figures

**Date:** 2026-11-20
**Status:** Complete (2026-11-20)

## Outcome

Built as planned, on `feat/report-periods`, in four commits: the
figures put together in one place, the domain, with the GUI moved onto
them; `--since`'s time section on them; `--week` and `--month`; and
the docs.

Choices made while building:

- **One place is the domain** (`timesheet_figures.go`), not a new
  package: reading a period's logs needs only the domain's own
  `TimelogRepository`, through which `LastLogBefore` already reads, so
  no storage and no config. `CalendarDate` (a date or a month as a
  day at noon), `ReadDays` (each date's log as a timesheet, a day to
  come unread), `SoFar` (the days up to today), and `Week` and `Month`
  (`WeekFigures`, `MonthFigures`: every figure either shows, over the
  days so far) were the GUI's own assembly; `CategoryTable`,
  `Stretch.PerWeek`, `Summary.UncategorisedShare` and
  `Categoriser.Tagged` hold the sums that were in its `tableOf` and
  `summaryOf`. The GUI only turns them into JSON (its golden week and
  month didn't change) and the report only lays them out. No package
  was added, so the architecture test has no new rules.
- **`--week` and `--month` are switches and the date is the
  command's argument**, as decision 1 has it: `htt report --week
  2026-11-04`. `--week=2026-11-04` isn't taken (pflag reads a
  boolean there). Errors: `--week and --month exclude each other: give
  one` (and so on with `--since`), `"2026-11-02": htt report takes a
  date only after --week (YYYY-MM-DD) or --month (YYYY-MM)`, `--week
  takes one date, got 2: …`, `--week: want a date, YYYY-MM-DD, got
  "…"`, `--month: want a month, YYYY-MM, or a date in it, YYYY-MM-DD,
  got "…"`.
- **Whether `--since` was given** is its flag value's own record
  (`givenString`), reset between the REPL's commands with the other
  flags: pflag's `Changed` stays set across them, so `/report --since
  2w` then `/report --week` would have been refused.
- **Layout.** The heading: `Activity in the week 2026-11-16 to
  2026-11-22 (so far: 5 of 7 days)`, `Activity in October 2026`, and
  `(to come)` for a period that hasn't started. The week: `--since`'s
  day lines and totals (`printDays`, shared), then the summary as a
  label and a value a line. The month: the summary, then three tables
  (weeks; weekdays; categories), names on the left and figures on the
  right, two spaces apart. The category table lists the categories
  with time, as the report leaves out lines without. A figure there
  isn't (an average over no days, the share of a category that isn't
  work, a half without days yet, a change from nothing) is `-`.
  Percentages are whole, rounded, as the page shows them; a change
  has its sign.
- **Durations** are the report's: hours and minutes, the minutes
  rounded down. A period's figure is first rounded to the second, as
  the API gives it, so the report and the API agree to the minute
  even for an average (`reportFigure`). The page writes durations the
  same way (its `dur` rounds the minutes down too, changed at review),
  so on screen the two read alike: July 2026's average week, 41h 32m
  54s, is "41h 32m" in both, which the browser test checks against the
  report's own output.
- **The clock** is to the second for `--week` and `--month`, as
  `htt gui`'s.
- **They read before they print**: an unreadable log or a config htt
  can't use stops `--week` and `--month` with nothing on stdout.
  `--since` still prints its tasks before reading the logs, as it did.
- **The tasks** run from the period's first day to the end of its
  last, or to now; a period to come has none.
- **Colour**: the report has never coloured anything, so there's
  nothing for `--no-color` to turn off, and the modes add none.

Deviations:

- **`--since`'s output changed in one case.** Where the clocks go
  forward at midnight (`America/Santiago`, 2026-09-06), its walk from
  midnight to midnight read the day before twice and listed it twice,
  half of a known issue in `TODO.md`. Read by date, it lists each day
  once. The other half, the timesheet of that night's day ending an
  hour early, is the domain's, and stays. An unreadable log's error
  now says `load the timelog for DATE` or `read the timelog for
  DATE`, as the GUI's does. Otherwise the output is the same: the old
  tests pass untouched, and the old and new binaries print the same
  report for nine `--since` values over 40 days of generated logs.

Tests, as planned:

- **The cross-check** (`TestReportPeriods_TheSameFiguresAsTheGUI`):
  the real binary's `htt report` and `htt gui`'s API on the same logs
  (the past week of 2 November, October with a day without `@end`,
  this week and this month with today running and a log for tomorrow
  written by hand, and a month to come). The report's heading and time
  section are read into rows of fields (split at two or more spaces),
  and the API's figures are put into the same rows, formatted as the
  report formats them; the two must be equal, every day, category,
  total, figure, week, weekday and table cell. htt has no clock to fix
  from outside, so the report runs between two fetches of the API: if
  those differ (a minute turned, today's entry running), it tries
  again; every figure only grows or shrinks with the time, so the
  report's, between them, must be theirs.
- **Golden text**, colour off: a past week and a past month in full
  (tasks and time), a month whose default category isn't work, a week
  and a month without time. The flags' exclusions and errors, the
  default periods and a period to come, a bad config and an unreadable
  log, and the REPL (each command's flags reset).
- **Domain**: `CalendarDate`, `ReadDays`, `SoFar`, `Week`, `Month`,
  `CategoryTable`, `UncategorisedShare`, `PerWeek`, `Tagged`, written
  first and seen failing against stubs.
- **Mutations**: breaking each piece of the new wiring in turn, 41
  mutations, failed a test every time. In the report: its own sums
  and averages (the week's total without today, the average week over
  all the month's days) and its own cut (the month's summary over all
  its days, "so far" as the whole period), caught by the cross-check;
  a Sunday read into the next week, caught by the week's golden; a
  figure from the wrong field (eight of them: breaks, a week's breaks,
  a weekday's average, the second half's rate, the time to midnight,
  the average working day, the change's sign, the uncategorised
  share), caught by the goldens and the cross-check; the rounding to
  the second, by `TestReportFigure`; the tasks' span, the heading, the
  flags and the REPL's reset, by their e2e tests; `--since`'s days and
  total, by its existing tests. In the domain: weeks from Sunday, a
  cut at tomorrow or none, the month's weekdays, halves and the week's
  summary over all their days, rolling weeks that don't reach back,
  days to come read, a share for a category that isn't work or for
  uncategorised time that isn't worked, a change from nothing,
  untagged categories listed, dates at midnight, a rate over one day
  too many: caught by the domain's and the GUI's tests, and most by
  the report's goldens too. One mutation was equivalent: the week's
  day lines over all seven days (days to come have no time, so aren't
  listed either way).

Left open:

- **`--week=DATE`** isn't taken; a flag that takes an optional value
  could take both forms, if wanted.
- **The night the clocks go forward at midnight** still ends a
  timesheet an hour early (`TODO.md`).

## Motivation

`htt gui` (docs/plans/complete/2026-11-19-gui-time-log.md) shows a
week and a month of the time log with figures computed in the domain
(`internal/domain/timesheet_period.go`: periods, totals, weeks, halves,
rolling totals, weekday averages, the summary). `htt report` computes
its own time section, and only for "since a date". Brice asked for the
report to reuse the GUI's figures, so the terminal and the browser
always agree, and the wishlist's calendar modes (`--week`, `--month`)
come with it.

## Decisions

1. **`htt report --week [DATE]` and `htt report --month [MONTH]`.**
   - DATE is YYYY-MM-DD: the Monday-to-Sunday week holding it.
   - MONTH is YYYY-MM, or a YYYY-MM-DD within it: that calendar month.
   - Without a value: this week or this month.
   - Dates are read as calendar dates, as the GUI's API reads them.
   - `--week`, `--month` and `--since` exclude each other.
   - The positional argument is accepted only with `--week` or `--month`; anywhere else it's an error that says so.
2. **The tasks sections** (completed, added, deleted) cover the period:
   from its first day to its last, or to now for the current one.
3. **The week's time section:** a line per day with worked time and
   time per category, as `--since` prints them now (days without `@end`
   flagged, today in progress), then the GUI's week summary: worked,
   breaks, average working day, counted to midnight, days without
   `@end`. For the current week every figure is over the days so far,
   as in the GUI.
4. **The month's time section:** the GUI's month summary (worked,
   average week, average working day, breaks, uncategorised share of
   worked, days without `@end`); the weeks (each Monday-to-Sunday week
   cut at the month's edges, with its days, worked and breaks); the
   average weekday (Monday to Sunday: average worked, of how many
   days); and the category table (total, share of worked, per week in
   days 1–15 and 16–end, change). No per-day lines and no rolling
   trends: `--since` and `htt gui` show those.
5. **One source of figures.** Every number in `--week` and `--month`
   comes from the domain functions the GUI uses; `--since`'s time
   section moves onto them too (its output unchanged), and the report's
   own day-walking and summing goes. Where the GUI and the report need
   the same assembly (loading a period's timesheets, the current-period
   "so far" cut), it lives in one place both call.
6. **Layout and colour** follow the report's existing style (aligned
   columns, the same duration format, colour off with `--no-color` or
   when not a terminal).

## Testing

- The cross-check: for a fixed set of time logs and a fixed now, the
  week and month figures `htt report` prints equal those the GUI's API
  returns, field by field (e2e, both from the real binary; past and
  current periods, a month with a day without `@end`).
- e2e for each section's text (golden output, colour off), the flags'
  exclusions and errors, the default periods, `--since` unchanged
  (its existing tests pass untouched).
- Mutation checks on the new wiring.

## Out of scope

- Rolling trends in text, other periods (quarters, years), CSV or JSON
  output from the report.
