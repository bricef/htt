# Time log: logging an entry at a given time (`--at`)

**Date:** 2026-11-18
**Status:** Planned

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
