# Time log: categories, breaks and day ends

**Date:** 2026-11-18
**Status:** Complete (2026-11-18)

## Outcome

Shipped as planned, with Brice's two clarifications (decisions 7 and
8). Choices made while building:

- **The model is `Timelog.Timesheet(categoriser, now)`** in the
  domain: a day's `Interval`s (entry, category, start, end), a status
  (`empty`, `ended`, `in progress`, `unended`), the final `@end`'s
  time, `Worked`, `ByCategory` and `Current`. `now` pins the clock and
  its zone is the local one, so tests fix both. `Latest`, `Duration`
  and `Spans` are gone; the CLI reads the timesheet instead.
- **An `@end` is an entry whose text is exactly `@end`** (annotations,
  a priority and dates aside), as `log end` writes it. An entry that
  only carries the tag among other words ("fix the @end bug +htt", as
  `workon` could log from a task) is ordinary, so it can't end a day
  by accident.
- **Entries are read in time order** (a stable sort, so entries
  logged in the same second keep file order and the earlier ones run
  for no time): a hand-edited file out of order still adds up.
- **Intervals are clamped to their day.** An entry stamped before the
  day's local midnight (a hand edit, or a change of time zone since)
  counts from midnight; one stamped after the day counts nothing; and
  today nothing counts after now. The day is the log's date in the
  local zone, 23 or 25 hours long when the clocks change.
- **Config checks.** Names and tags compare exactly, like the tags
  themselves (decision 7), so `Break` and `break` are two categories.
  A tag needs its sigil and no spaces (an entry's tags always carry
  their sigil, so a bare word could never match), and can be in one
  category only (a second listing could never match; listing it twice
  in one category is harmless). `work` must be `true` or `false` and
  defaults to `true`. Unknown keys, in the `timelog` section or a
  category, are errors, and so is a map where the list goes (the error
  says order matters). An empty tag left by an unquoted `#` says to
  quote it. Errors name the key, then the category by its position
  counting from 1 and its name: `config: timelog.categories:
  category 3 (admin): no tags, and it isn't the default category, …`.
  A category without tags is allowed when it's the default (amended at
  review: Brice said the default may be "some other defined category",
  and one that exists only to be the default has nothing to match).
- **Only `htt report` reads the categories.** `log status`,
  `log active`, `htt status` and the commands that write entries
  don't, so a bad `timelog` config stops the report (before it prints
  anything) and nothing else, as a bad theme stops only
  `htt interactive`.
- **A fresh config gets `timelog: default_category: working`**, as it
  gets `tui.theme`, so `htt config yaml` shows the setting; the domain
  falls back on `working` anyway.
- **The report's time section** keeps its heading and
  `formatReportDuration`: a line per day, `2026-11-04  15h 0m worked
  (no @end: counted to 24:00)` or `… (in progress)` for today, then a
  line per category with time, names lined up on the left and
  durations on the right across the section; then `Total: … worked`
  and the categories' totals in the same layout. Categories come in
  the config's order, the default last unless it is one of them.
  Categories and days with no time are left out, except today in
  progress. Days are walked a calendar day at a time in the local
  zone: the old 24-hour step would have listed the day the clocks go
  back twice now that days are local.
- **`log status` and `log active`** keep their own wording when
  working ("Currently working on: …", "Working on: …") and when
  nothing is logged, and both say "Not working (day ended at 17:00)"
  after an `@end`. Status lost a trailing space. Neither names the
  category: that is an open question (below), though "What changes
  for users" assumed they would.

- **Tests** as planned: table tests per rule, an oracle over 3000
  generated days in five zones (clock changes included) that counts
  each minute on its own, a first-match property with near-miss tags,
  config tests through viper, and e2e tests with htt run in
  Asia/Kolkata. Breaking each core rule in turn (26 mutations: @end
  making an interval, the 24:00 and until-now rules, UTC or 24-hour
  days, sorting, clamping, work: false in worked time, first match,
  exact matching, each config check, the default category's rules,
  status after @end, the report's day walk) made at least one test
  fail every time.

Not built, needing a decision:

- **Decision 4 says `log status` says a day has no `@end`,** but
  status looks at today only, and today's open entry is in progress,
  never flagged. Flagging a past day would mean status looking back
  (to yesterday? the last day with entries?), which nothing decides,
  so only the report flags unended days.

What building showed about the open questions:

- **The category in `log status`** matters most on a break: it says
  "Currently working on: @lunch (20m)". Naming the category, or
  saying "On a break", would read better.
- **"Uncategorised"**: with exact matching, a misspelt or
  differently-cased tag (`+HTT`) silently lands in `working`. Listing
  the default category's time separately, or the tags that fell
  through, would catch that.
- **`--at`**: a forgotten `log end` now counts the day's last entry
  until midnight (a 15-hour day in the report). The report flags it,
  but `htt log edit` is still the only fix.

Amendments: Brice's decisions of 2026-11-18, after the merge, on two
of the open questions above.

- **Uncategorised time has a line of its own in the report.** An
  entry no category's tags match is uncategorised: the report shows
  its time per day and in the totals on an `uncategorised` line, never
  folded into a named category's, so an unmapped or misspelt tag
  stands out. Whether it counts as worked still follows the default
  category (decision 8), and the line names it:
  `uncategorised  3h 0m  (counted as working)`, lined up with the
  categories' lines, after them. A category's line holds only the
  entries its tags matched, the default's included, so the built-in
  `working`, which has no tags, never has a line. The domain records
  it: `Categoriser.Match` says whether a category matched, each
  `Interval` carries `Uncategorised`, `ByCategory` is matched time
  only and `Timesheet.Uncategorised` the rest; `Worked` is unchanged.
- **Status names the category.** `log status`, `log active` and
  `htt status` read the categories, so "Only `htt report` reads the
  categories" above no longer holds. The running entry gets its
  category and how long it has run:
  `Currently working on: review the PR +htt (htt, 1h0m)` (`Working on:
  …` from `log active`); `Not working: @lunch (break, 12m)` from both
  when the category doesn't count as work; and `uncategorised` when no
  category matched, working or not as the default category says. The
  lines after an `@end` and on a day with nothing logged don't change.
  A bad `timelog` config doesn't stop them: they warn on stderr
  (`warning: config: timelog.…: …; showing entries without their
  categories`), keep the exit code, and show the entry as before,
  without its category. They warn only when they show an entry, since
  no other line has a category to lose.

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
   work (but see 8). `workon` copies a task's text, tags included, so
   its time is categorised by the task's `+project`.
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

Clarified by Brice during the build (2026-11-18):

7. **Tag matching is exact.** A configured tag matches only an
   identical tag in the entry: same characters, same case; no prefix,
   partial, fuzzy or case-insensitive matching. `@break` matches an
   entry with the tag `@break` (`@break coffee`), and not `@Break`,
   `@breakfast` or `@break-time`.
8. **`default_category` may name any category in
   `timelog.categories`,** and that category's `work` setting then
   decides whether unmatched time counts as work. Unset, the default
   is `working`, the built-in category that counts as work; if the
   list defines `working` itself, the list's definition (its `work`
   included) applies. Naming a category that isn't defined (other than
   `working`) is a config error naming the key and the value.

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
  Decided: it does (see Amendments in Outcome).
- Should the report list time in the default category separately as
  "uncategorised" too, so unmapped tags are noticed? Decided: yes (see
  Amendments in Outcome).
