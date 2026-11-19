# GUI: the time log, seen

**Date:** 2026-11-19
**Status:** Planned

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
2. **Local and private.** It listens on 127.0.0.1 only. Every path
   starts with a random token (128 bits, new each run) that the printed
   and opened address carries; anything else is a 404. A request whose
   `Host` isn't `127.0.0.1:<port>` or `localhost:<port>` is refused
   (DNS rebinding). It only reads: GET and HEAD only, nothing writes.
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
- `internal/gui` (new): the server (listener, token, Host check,
  shutdown), handlers for the week, the month and the page's assets,
  JSON view models, and the embedded assets. It imports `domain`; the
  CLI hands it a timelog repository, a categoriser and a clock, so it
  imports neither `cli` nor `storage`. The architecture test gains its
  rules: `gui` doesn't import `cli` or `tui`; `cli` may import `gui`.
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
  fixed clock: the JSON for a known week and month (golden), token and
  Host checks, methods, bad dates, a bad config.
- **e2e**: the real binary with `htt gui --no-open --port 0`: read the
  printed address, fetch the week and month, check a wrong token is a
  404, then stop it with an interrupt and see it exit cleanly.
- **Browser smoke test**: headless Chrome (chromedp) loads the page from
  the real binary and checks it draws both views without a JavaScript
  error; skipped where no Chrome is installed. CI's runners have one.
- **By eye**: screenshots of both views, light and dark, desktop and
  phone width, in the report back.

## Out of scope

- A native window (Wails v3 can wrap the same page later), editing
  entries, task views, colours in the config, push updates (SSE).
