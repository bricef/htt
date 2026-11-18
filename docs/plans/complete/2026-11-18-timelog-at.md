# Time log: logging an entry at a given time (`--at`)

**Date:** 2026-11-18
**Status:** Complete (2026-11-18)

## Outcome

Shipped as planned. Choices made while building:

- **`dates.ParseAt(input, now)` reads the time.** `H:MM` works in all
  three forms (`yesterday 7:05`, `2026-11-10 8:15`), and any spaces
  between the words. Hours run 0 to 23 (`24:00` is refused), minutes
  take two digits, and seconds aren't accepted. `yesterday` is taken
  as written: `Yesterday 17:30` is an error, like `today 9:30`. A date
  is `YYYY-MM-DD`, zero-padded and real (`2026-02-30` is refused).
  Yesterday is the calendar day before now's, not 24 hours back, and a
  date is read for its year, month and day alone, so neither slips a
  day where the clocks change at midnight. The error quotes the input
  and lists the forms: `--at: "9.30" isn't a time htt can read (use
  HH:MM for today, "yesterday HH:MM" or "YYYY-MM-DD HH:MM", on the
  24-hour clock)`. The parser doesn't refuse a later time; the time
  log does.
- **What `time.Date` does on a day the clocks change** (decision 2),
  pinned by the tests: it differs by zone. In London, 01:30 on the day
  the clocks go forward is 02:30 BST, and 01:30 on the day they go
  back is the second one, in GMT. In New York, 02:30 on the day they
  go forward is 01:30 EST, and 01:30 on the day they go back is the
  first, in EDT. In Santiago, whose clocks go forward at midnight,
  00:30 that day is 23:30 the day before, and is logged into that
  day's file. The confirmation shows the time it became (`(at 02:30
  on 2026-03-29)`), so a shift is visible.
- **The domain places the entry: `Timelog.AppendAt(task, at, now)`.**
  It goes right after the last entry logged at or before its time, or
  first: between its neighbours in a day in time order, and after an
  entry at the same time, so the new one is the one that runs (as for
  entries logged in the same second). The entries already there keep
  their order, even out of time order after a hand edit. Its `ts:` is
  the time in UTC, to the second. It refuses, changing nothing, a
  time later than now (`ErrFutureEntry`), one whose date in now's
  zone isn't the log's, and a day with an entry whose `ts:` it can't
  read, since it can't place the new one against it. The day is
  checked by date rather than from midnight to midnight because of
  the Santiago case: `time.Date` puts a midnight that doesn't exist on
  the day before. `Append` is unchanged, and still appends at the end.
- **`--at` on the command line.** An empty `--at ""` is refused like
  any time htt can't read, not taken as now (the flag records that it
  was given). The four commands share one flag variable, which the
  REPL clears between commands. The confirmation names the day by the
  time's date, whichever form gave it: `(at 09:30 today)`, `(at 17:30
  yesterday)`, `(at 08:15 on 2026-11-10)`. A later time: `--at
  "23:59" is later than now (18:54): nothing logged`; nothing is
  written, not even a backup. The day's log is loaded before that
  check, so it warns about dates in that day's entries as `log add`
  always has. A typed entry's date warning names the day's file and
  the line the entry went to.
- **The report's fix is on a line of its own** (decision 6 left the
  choice), under the day's line and lined up with its time:

  ```
    2026-11-17  15h 0m worked (no @end: counted to 24:00)
                end the day with htt log end --at "2026-11-17 HH:MM"
  ```

  On the day's line it made a line of about 105 columns.
- **Help.** Each command's help ends with the same paragraph on
  `--at`; `log end`'s shows the forgotten-end fix; the flag's own line
  is short (`--at time   log it at HH:MM (today), "yesterday HH:MM" or
  "YYYY-MM-DD HH:MM"`).
- **Tests**: table tests for each form, edge values and 45 rejected
  inputs; yesterday across months, years and clock changes; the clock
  changes above; AppendAt's placement, ties, stamp, refusals (nothing
  changed, nothing saved, the entry not stamped) and day bounds, the
  forgotten end it fixes, and a property test over 1000 generated
  days (out-of-order and off-day entries included, five zones) that
  inserting at random times keeps a day in order, loses and reorders
  nothing, and stamps and saves the entry; e2e tests for each scenario
  in the plan, the clock changes, typed-entry warnings, help and the
  REPL, run in Kolkata and skipped within ten minutes of midnight.
  Breaking each rule in turn (34 mutations: each parsing rule, the
  future rule both ways, the day check, its zone and the midnight-to-
  midnight version, placement at ties, at the end and before the
  first later entry, the UTC stamp, saving, bad timestamps, stamping
  before the checks, the confirmation, today and yesterday, loading
  the right day, an empty `--at`, the flag on each command, the REPL
  reset, and the report's fix line, its date and the days it goes on)
  made a test fail every time.

Decided after the merge (Brice, 2026-11-18):

- **The REPL reads quotes as the shell does.** At first it split a
  line at its spaces and didn't read quotes, so `/log end --at
  "yesterday 17:30"` was an error there. Brice chose to reconcile the
  REPL with the terminal: quotes in task text are unusual, and the
  shell strips them too. A `"` or `'` at the start of a word, or after
  a `=`, opens a quoted part that closes at the next quote of its kind
  ending a word; an apostrophe inside a word (`Sam's`, `kids'`) or a
  quote nothing closes stays as typed, so ordinary task text doesn't
  change meaning or fail, where the shell would wait for a closing
  quote. Lines without quotes split as before (`replWords` in
  internal/cli/repl_words.go).
- **`--at` takes ISO 8601 too** (Brice asked for it): the extended
  form `YYYY-MM-DDTHH:MM`, then `:SS` (a fraction of a second is
  dropped, as the log's `ts:` has none) and a zone (`Z`, `±HH:MM` or
  `±HHMM`), each optional. Without a zone it's local time, as ISO 8601
  reads it, and as the `"YYYY-MM-DD HH:MM"` form on a day the clocks
  change; with one it's that instant, in whichever local day it falls,
  so a `ts:` copied from the log works and a repeated 01:30 can be
  told apart. Seconds are kept when given, which overrides decision
  2's "seconds zero" for this form only, and the confirmation then
  shows them (`at 10:30:15 yesterday`). Not taken: the basic form
  (`20261110T0930`), an hour-only zone (`+05`), a lowercase `t` or
  `z`, and `H:MM` after the `T`.

Found while building, not fixed (it predates `--at`, and is in
TODO.md's known issues): where the clocks go forward at midnight, as
in Santiago, the report gets that night wrong. `time.Date` puts the
missing midnight an hour early, so `Timesheet` ends the day before at
23:00 and the report's day walk lists that day twice.

## Motivation

Every time-log entry is stamped with the moment it's logged. Since the
time log's day rules (docs/plans/complete/2026-11-18-timelog-categories.md),
a day without an `@end` counts its last entry to 24:00 and is flagged,
so a forgotten `htt log end` turns into a 15-hour day. The only fix
today is editing the log file by hand. The same goes for starting late
("I've been on this since 9:30") or a break you forgot to log. This is
the wishlist's `log@`.

## Decisions

1. **`--at` on every command that writes an entry:** `htt log start`,
   `htt log add`, `htt log end` and `htt workon`.
2. **What it takes**, all 24-hour local time, seconds zero:
   - `HH:MM` (or `H:MM`): today at that time;
   - `yesterday HH:MM`: yesterday at that time;
   - `YYYY-MM-DD HH:MM`: that day at that time.

   Anything else is an error saying which forms work. A time that
   doesn't exist or is ambiguous on a day the clocks change follows
   Go's `time.Date` (and the tests pin what that does).
3. **No future times:** an `--at` later than now is refused.
4. **The entry goes into its own day's file, in time order:** `--at
   "yesterday 17:30"` writes to yesterday's log, and an entry at 14:00
   lands between the 13:00 and 15:00 entries, so `htt log show` reads
   in order. Its `ts:` is that time (in UTC, as all entries).
5. **The confirmation says when:** `Logging entry: @end (at 17:30
   yesterday)` or similar, so a wrong `--at` is obvious; without
   `--at`, output is unchanged.
6. **The report's unended-day flag gives the fix:** e.g.
   `2026-11-17  15h 0m worked (no @end: counted to 24:00; end it with
   htt log end --at "2026-11-17 HH:MM")`, or the same advice on a line
   of its own if that reads better.
7. **Nothing else changes:** without `--at` every command behaves as
   now, and the file format is unchanged.

## Testing

- Parsing: each accepted form, edge values (00:00, 23:59), rejected
  forms (25:00, 9.30, words, a date alone), the future rule (with an
  injected now), and a day the clocks change.
- Domain: inserting an entry at a time keeps a day's entries in time
  order (property test over random insertions); the entry's `ts:` is
  the given time.
- e2e: fixing a forgotten end (`log end --at "yesterday 17:30"` turns
  yesterday's flagged day into an ended one in the report); a late
  start; an entry inserted mid-day appears in order in `log show`;
  `workon --at`; a future `--at` is refused with nothing written; the
  confirmation text; the report's flag text.

## Out of scope

- Moving or deleting an existing entry (`htt log edit` remains).
- Natural-language times beyond `yesterday` ("5pm", "an hour ago").
