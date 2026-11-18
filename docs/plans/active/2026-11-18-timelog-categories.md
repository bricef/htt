# Time log: categories, breaks and day ends

**Date:** 2026-11-18
**Status:** Planned

## Motivation

The time log is one file per day; each entry means "from now on I'm
doing X", and its time runs until the next entry. `@start` and `@end`
are entries like any other, which goes wrong in four ways (checked on
main, 085061e, with a hand-written day):

- After `htt log end`, `log status` and `log active` report "Currently
  working on: @end".
- Time after an `@end` is counted: a day of 9:00–12:00 and 13:00–17:00
  with lunch marked by `log end` reports 8h, not 7h, the lunch hour
  credited to "@end".
- A day's last entry gets no duration unless something follows it, so
  without `log end` its time is silently lost.
- Nothing classifies time: a report can only total everything.

The next step for the time log is a visualisation (a timeline of each
day); this plan gives it the model it needs.

## Decisions captured

From the design discussion (2026-11-18):

1. **Breaks are explicit entries,** such as `@break` or `@lunch`, not
   `@end`. Whether an entry is a break is decided by its category.
2. **A category map in the config classifies entries.** It matches the
   entry's todo.txt tags (`+project`, `@context`, `#tag`), and it is a
   list, so the first category with a matching tag wins when an entry
   has several:

   ```yaml
   timelog:
     categories:            # first match wins
       - name: break
         tags: ["@break", "@lunch"]
         work: false
       - name: htt
         tags: ["+htt"]
       - name: admin
         tags: ["@email", "+admin"]
     default_category: working   # entries no category matches
   ```

   `work: false` keeps a category out of worked time; categories count
   as work unless they say otherwise. Entries that match nothing go to
   the default category (`working` unless configured), which counts as
   work. `workon` copies a task's text, tags included, so its time is
   categorised by the task's `+project`.
3. **`@end` ends the day.** It closes the last entry. An entry logged
   after it starts a new stretch, and the gap between them counts as
   nothing (so existing logs that used `@end` for lunch read sensibly).
4. **A day that doesn't end runs to 24:00 and is flagged.** Its last
   entry counts until midnight (local time of that day), and the
   report and `log status` say the day has no `@end`. The visualisation
   will make such days obvious.
5. **`@start` is an ordinary entry,** categorised like any other (the
   default category unless mapped): the time between it and the first
   real entry is unattributed work.
6. **The file format doesn't change.** Existing logs are read with the
   new rules, so past reports become correct retroactively.

## The model

- An **interval** is an entry, its start (its `ts:`), its end, and its
  category. An entry's end is the next entry's time; for the last
  entry before an `@end`, the `@end`'s time; for a day's final entry,
  the day's `@end` or, failing one, 24:00 local (flagged), except
  **today**, whose final entry runs until now and is reported as in
  progress, not flagged.
- `@end` entries make no interval; the gap after one, until the next
  entry, belongs to no interval.
- **Worked time** is the sum of intervals whose category counts as
  work. Per-category time is the sum per category, breaks included.
- Days stay independent: an interval never crosses midnight. (An entry
  logged before midnight and not ended runs to 24:00; the next day
  starts with its own first entry.)
- Timestamps are stored in UTC; day boundaries and "24:00" use the
  local time zone, as day files already do.
- The domain stays config-free: the CLI reads `timelog.*` and builds a
  categoriser (as it builds the TUI's theme), and invalid config is an
  error naming the key and the problem, like `tui.colors`.

## What changes for users

- `htt log status` / `htt log active` (both look at today only): after
  `@end`, "Not working (day ended at 17:00)"; otherwise the current
  entry, its category and how long it has run.
- `htt report`'s time section: per day, worked time and time per
  category, with unended days flagged; a total of worked time and per
  category over the period. The "(excluding currently-active entry)"
  caveat goes: today's open entry counts until now, marked in progress.
- `log end`'s help and the docs say it ends the day; breaks are logged
  as entries (`htt log add @lunch`).
- GETTING-STARTED's time-log section explains categories, breaks, day
  ends and the 24:00 rule; TODO.md's `@end` known issue and the
  "open/closed entries" wishlist item go to Done.

## Testing

- Domain: an oracle for intervals over generated days (random entries,
  `@end`s, with and without a final `@end`, today and past days): the
  intervals are ordered, don't overlap, cover exactly the time from
  each stretch's first entry to its close, and their sum per category
  matches a direct computation; first-match-wins categorisation over
  generated tag sets.
- Config: valid maps, unknown keys, empty names, duplicate names, a
  category with no tags, `work` defaults.
- e2e: the 9–12 / lunch / 13–17 day reports 7h worked and 1h break;
  an unended past day runs to 24:00 and is flagged; today's open entry
  runs to now; `log status` after `@end`; the existing time-log
  commands and their outputs elsewhere unchanged.

## Out of scope

- The visualisation (the next step).
- Time per todo task (wishlist).
- Recording an entry at a past time (`--at`; see open questions).

## Open questions

- An `--at HH:MM` option on `log start`, `log add` and `log end`, to
  record an entry after the fact (the wishlist's `log@`): with the
  24:00 rule, a forgotten `@end` is otherwise fixed only by editing the
  file.
- Should `log status` name the category, or only the report?
- Should the report list time in the default category separately as
  "uncategorised" too, so unmapped tags are noticed?
