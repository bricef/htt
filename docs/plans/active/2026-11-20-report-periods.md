# Report: a week or a month, with the GUI's figures

**Date:** 2026-11-20
**Status:** Planned

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
