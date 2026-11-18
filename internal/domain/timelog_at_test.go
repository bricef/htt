package domain

import (
	"errors"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
)

// recordingTimelogRepository is a TimelogRepository that records what
// each Save is given, for AppendAt's tests. The other methods panic.
type recordingTimelogRepository struct {
	stubTimelogRepository
	saves [][]string // each Save's entries, by Raw
}

func (r *recordingTimelogRepository) Save(l *Timelog) error {
	saved := make([]string, len(l.Entries))
	for i, entry := range l.Entries {
		saved[i] = entry.Raw
	}
	r.saves = append(r.saves, saved)
	return nil
}

// wiredDayLog is dayLog, wired to a recording repository.
func wiredDayLog(t *testing.T, loc *time.Location, date string, lines ...string) (*Timelog, *recordingTimelogRepository) {
	t.Helper()
	repo := &recordingTimelogRepository{}
	built := dayLog(t, loc, date, lines...)
	l := NewTimelog(repo, built.Date)
	l.Entries = built.Entries
	return l, repo
}

// entryTexts is each entry's text, without its ts:, in file order.
func entryTexts(l *Timelog) []string {
	texts := make([]string, len(l.Entries))
	for i, entry := range l.Entries {
		texts[i], _, _ = strings.Cut(entry.Raw, " "+TimelogTimestampLabel+":")
	}
	return texts
}

func TestTimelog_AppendAtPutsTheEntryInTimeOrder(t *testing.T) {
	loc := mustZone(t, "Asia/Kolkata")
	now := localTime(t, loc, "2026-11-18 18:00")
	day := []string{"09:00 @start", "13:00 review the PR +htt", "15:00 mail @email"}
	cases := []struct {
		at   string
		want []string
	}{
		{"14:00", []string{"@start", "review the PR +htt", "new", "mail @email"}},
		{"08:00", []string{"new", "@start", "review the PR +htt", "mail @email"}},
		{"00:00", []string{"new", "@start", "review the PR +htt", "mail @email"}},
		{"16:00", []string{"@start", "review the PR +htt", "mail @email", "new"}},
		{"18:00", []string{"@start", "review the PR +htt", "mail @email", "new"}},
		// At the same time as an entry, after it: logged later, it is
		// the one that runs.
		{"09:00", []string{"@start", "new", "review the PR +htt", "mail @email"}},
		{"13:00", []string{"@start", "review the PR +htt", "new", "mail @email"}},
		{"15:00", []string{"@start", "review the PR +htt", "mail @email", "new"}},
	}
	for _, c := range cases {
		t.Run(c.at, func(t *testing.T) {
			l, repo := wiredDayLog(t, loc, "2026-11-18", day...)
			entry := mustTask(t, "new")
			at := localTime(t, loc, "2026-11-18 "+c.at)
			if err := l.AppendAt(entry, at, now); err != nil {
				t.Fatalf("AppendAt(%s): %v", c.at, err)
			}
			if got := entryTexts(l); !slices.Equal(got, c.want) {
				t.Errorf("entries after AppendAt(%s) = %q, want %q", c.at, got, c.want)
			}
			if wantRaw := "new ts:" + at.UTC().Format(time.RFC3339); entry.Raw != wantRaw {
				t.Errorf("entry = %q, want %q", entry.Raw, wantRaw)
			}
			if len(repo.saves) != 1 {
				t.Fatalf("saved %d times, want once", len(repo.saves))
			}
			if want := rawEntries(l); !slices.Equal(repo.saves[0], want) {
				t.Errorf("saved %q, want %q", repo.saves[0], want)
			}
		})
	}

	// An empty day: the entry is the day's first.
	l, repo := wiredDayLog(t, loc, "2026-11-18")
	if err := l.AppendAt(mustTask(t, "new"), localTime(t, loc, "2026-11-18 10:00"), now); err != nil {
		t.Fatal(err)
	}
	if want := []string{"new ts:2026-11-18T04:30:00Z"}; !slices.Equal(rawEntries(l), want) || len(repo.saves) != 1 {
		t.Errorf("empty day after AppendAt = %q (%d saves), want %q saved once", rawEntries(l), len(repo.saves), want)
	}
}

func rawEntries(l *Timelog) []string {
	raws := make([]string, len(l.Entries))
	for i, entry := range l.Entries {
		raws[i] = entry.Raw
	}
	return raws
}

func TestTimelog_AppendAtStampsTheTimeInUTC(t *testing.T) {
	// ts: is the given time in UTC, to the second, as Append stamps
	// now; whatever zone the time comes in.
	loc := mustZone(t, "Asia/Kolkata")
	now := localTime(t, loc, "2026-11-18 18:00")
	for _, at := range []time.Time{
		localTime(t, loc, "2026-11-18 14:00"),
		localTime(t, loc, "2026-11-18 14:00").UTC(),
		localTime(t, loc, "2026-11-18 14:00").In(mustZone(t, "America/New_York")),
		localTime(t, loc, "2026-11-18 14:00").Add(999 * time.Millisecond),
	} {
		l, _ := wiredDayLog(t, loc, "2026-11-18")
		entry := mustTask(t, "+htt review")
		if err := l.AppendAt(entry, at, now); err != nil {
			t.Fatalf("AppendAt(%v): %v", at, err)
		}
		if got, want := entry.Annotations[TimelogTimestampLabel], "2026-11-18T08:30:00Z"; got != want {
			t.Errorf("AppendAt(%v): ts:%s, want ts:%s", at, got, want)
		}
	}
}

func TestTimelog_AppendAtRefusesALaterTime(t *testing.T) {
	loc := mustZone(t, "Asia/Kolkata")
	now := time.Date(2026, 11, 18, 15, 42, 30, 0, loc)
	day := []string{"09:00 @start", "13:00 review the PR +htt"}

	// Now itself is fine, and so is the minute before.
	for _, at := range []time.Time{now, now.Truncate(time.Minute)} {
		l, _ := wiredDayLog(t, loc, "2026-11-18", day...)
		if err := l.AppendAt(mustTask(t, "new"), at, now); err != nil {
			t.Errorf("AppendAt(%v) at %v: %v", at, now, err)
		}
	}
	// A moment later isn't, nor the next minute, nor another day.
	for _, c := range []struct{ date, at string }{
		{"2026-11-18", "2026-11-18 15:43"},
		{"2026-11-18", "2026-11-18 23:59"},
		{"2026-11-19", "2026-11-19 09:00"},
	} {
		l, repo := wiredDayLog(t, loc, c.date, "09:00 @start")
		before := rawEntries(l)
		entry := mustTask(t, "new")
		err := l.AppendAt(entry, localTime(t, loc, c.at), now)
		if !errors.Is(err, ErrFutureEntry) {
			t.Errorf("AppendAt(%s) at %v = %v, want ErrFutureEntry", c.at, now, err)
		}
		assertUnchanged(t, c.at, l, before, repo, entry)
	}
	l, repo := wiredDayLog(t, loc, "2026-11-18", day...)
	before := rawEntries(l)
	entry := mustTask(t, "new")
	if err := l.AppendAt(entry, now.Add(time.Second), now); !errors.Is(err, ErrFutureEntry) {
		t.Errorf("AppendAt(now + 1s) = %v, want ErrFutureEntry", err)
	}
	assertUnchanged(t, "now + 1s", l, before, repo, entry)
}

// assertUnchanged checks a refused AppendAt changed nothing: not the
// log's entries, not the entry, and nothing was saved.
func assertUnchanged(t *testing.T, label string, l *Timelog, before []string, repo *recordingTimelogRepository, entry *Task) {
	t.Helper()
	if got := rawEntries(l); !slices.Equal(got, before) {
		t.Errorf("%s: entries = %q after a refusal, want %q", label, got, before)
	}
	if len(repo.saves) != 0 {
		t.Errorf("%s: saved %q after a refusal", label, repo.saves)
	}
	if _, stamped := entry.Annotations[TimelogTimestampLabel]; stamped {
		t.Errorf("%s: the refused entry was stamped: %q", label, entry.Raw)
	}
}

func TestTimelog_AppendAtStaysOnTheLogsDay(t *testing.T) {
	// The day is the log's date in now's zone, midnight to midnight: 23
	// or 25 hours on a day the clocks change. Where they go forward at
	// midnight, the day before ends at midnight all the same, not an
	// hour early, where time.Date puts the next day's midnight.
	cases := []struct {
		zone, date string
		ok, not    []string // times on the day, and not
	}{
		{"Asia/Kolkata", "2026-11-17",
			[]string{"2026-11-17 00:00", "2026-11-17 12:00", "2026-11-17 23:59"},
			[]string{"2026-11-16 23:59", "2026-11-18 00:00", "2026-11-18 09:00", "2025-11-17 12:00"}},
		{"Europe/London", "2026-10-25", // 25 hours
			[]string{"2026-10-25 00:00", "2026-10-25 01:30", "2026-10-25 23:59"},
			[]string{"2026-10-24 23:59", "2026-10-26 00:00"}},
		{"Europe/London", "2026-03-29", // 23 hours
			[]string{"2026-03-29 00:00", "2026-03-29 02:00", "2026-03-29 23:59"},
			[]string{"2026-03-28 23:59", "2026-03-30 00:00"}},
	}
	for _, c := range cases {
		loc := mustZone(t, c.zone)
		now := localTime(t, loc, "2026-11-18 18:00")
		for _, at := range c.ok {
			l, _ := wiredDayLog(t, loc, c.date)
			if err := l.AppendAt(mustTask(t, "new"), localTime(t, loc, at), now); err != nil {
				t.Errorf("%s: AppendAt(%s) to %s: %v", c.zone, at, c.date, err)
			}
		}
		for _, at := range c.not {
			l, repo := wiredDayLog(t, loc, c.date, "09:00 @start")
			before := rawEntries(l)
			entry := mustTask(t, "new")
			err := l.AppendAt(entry, localTime(t, loc, at), now)
			if err == nil || errors.Is(err, ErrFutureEntry) {
				t.Errorf("%s: AppendAt(%s) to %s = %v, want an error that it isn't the log's day", c.zone, at, c.date, err)
			}
			assertUnchanged(t, c.zone+" "+at, l, before, repo, entry)
		}
	}

	// Santiago's clocks go forward at midnight on 2026-09-06: the 5th
	// runs to 23:59 at -04, the 6th starts at 01:00 at -03.
	santiago := mustZone(t, "America/Santiago")
	now := localTime(t, santiago, "2026-11-18 18:00")
	for _, c := range []struct {
		day int // of September
		at  time.Time
		ok  bool
	}{
		{5, time.Date(2026, 9, 6, 3, 30, 0, 0, time.UTC), true},  // 23:30 -04
		{5, time.Date(2026, 9, 6, 4, 0, 0, 0, time.UTC), false},  // 01:00 -03 on the 6th
		{6, time.Date(2026, 9, 6, 4, 0, 0, 0, time.UTC), true},   // 01:00 -03
		{6, time.Date(2026, 9, 6, 3, 59, 0, 0, time.UTC), false}, // 23:59 -04 on the 5th
	} {
		l := NewTimelog(&recordingTimelogRepository{}, time.Date(2026, 9, c.day, 12, 0, 0, 0, santiago))
		err := l.AppendAt(mustTask(t, "new"), c.at, now)
		if (err == nil) != c.ok {
			t.Errorf("Santiago: AppendAt(%v) to 2026-09-%02d = %v, want ok %v", c.at.In(santiago), c.day, err, c.ok)
		}
	}

	// The same instant is on the day or not depending on the zone: the
	// log's date is read in now's zone. 20:00 UTC on the 17th is the
	// 18th in Kolkata.
	l, _ := wiredDayLog(t, time.UTC, "2026-11-17")
	at := time.Date(2026, 11, 17, 20, 0, 0, 0, time.UTC)
	if err := l.AppendAt(mustTask(t, "new"), at, time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Errorf("in UTC, AppendAt(%v) to 2026-11-17: %v", at, err)
	}
	kolkata := mustZone(t, "Asia/Kolkata")
	if err := l.AppendAt(mustTask(t, "new"), at, time.Date(2026, 11, 18, 12, 0, 0, 0, kolkata)); err == nil {
		t.Errorf("in Kolkata, AppendAt(%v) to 2026-11-17 = nil, want an error", at)
	}
}

func TestTimelog_AppendAtNeedsTheEntriesTimes(t *testing.T) {
	// An entry without a ts: (a hand edit) can't be placed against.
	loc := mustZone(t, "Asia/Kolkata")
	for _, bad := range []string{"no time", "bad time ts:yesterday"} {
		l, repo := wiredDayLog(t, loc, "2026-11-17", "09:00 @start")
		l.Entries = append(l.Entries, mustTask(t, bad))
		before := rawEntries(l)
		entry := mustTask(t, "new")
		err := l.AppendAt(entry, localTime(t, loc, "2026-11-17 10:00"), localTime(t, loc, "2026-11-18 12:00"))
		if err == nil {
			t.Errorf("AppendAt with an entry %q = nil, want an error", bad)
		}
		assertUnchanged(t, bad, l, before, repo, entry)
	}
}

func TestTimelog_AppendAtEndsAForgottenDay(t *testing.T) {
	// The forgotten log end: a past day that ran to 24:00 ends where
	// the @end goes, if it goes after the day's last entry.
	loc := mustZone(t, "Asia/Kolkata")
	now := localTime(t, loc, "2026-11-18 10:00")
	day := []string{"09:00 @start", "09:30 review the PR +htt", "13:00 mail @email"}

	l, _ := wiredDayLog(t, loc, "2026-11-17", day...)
	sheet, err := l.Timesheet(nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Status != DayUnended || sheet.Worked() != 15*time.Hour {
		t.Fatalf("before: %v, %v worked; want unended, 15h", sheet.Status, sheet.Worked())
	}
	if err := l.AppendAt(mustTask(t, DayEndEntry), localTime(t, loc, "2026-11-17 17:30"), now); err != nil {
		t.Fatal(err)
	}
	sheet, err = l.Timesheet(nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Status != DayEnded || !sheet.EndedAt.Equal(localTime(t, loc, "2026-11-17 17:30")) || sheet.Worked() != 8*time.Hour+30*time.Minute {
		t.Errorf("after @end at 17:30: %v at %v, %v worked; want ended at 17:30, 8h30m", sheet.Status, sheet.EndedAt, sheet.Worked())
	}

	// An @end before the last entry ends a stretch, not the day.
	l, _ = wiredDayLog(t, loc, "2026-11-17", day...)
	if err := l.AppendAt(mustTask(t, DayEndEntry), localTime(t, loc, "2026-11-17 12:00"), now); err != nil {
		t.Fatal(err)
	}
	sheet, err = l.Timesheet(nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Status != DayUnended || sheet.Worked() != 14*time.Hour {
		t.Errorf("after @end at 12:00: %v, %v worked; want unended, 14h", sheet.Status, sheet.Worked())
	}
}

// TestTimelog_AppendAtProperty logs entries at random times into
// generated days (generateDay: random entries, a few out of order or
// off the day as hand edits leave them, today and past days, in zones
// with clock changes), checking after each that the entry went right
// after the last entry at or before its time (or first), so every
// entry after it is later, and a day in time order stays in order;
// that nothing was lost or reordered: the entries are those before,
// in the same order, with the new one added; that its ts: is its time;
// that the saved entries are the log's; and that the timesheet runs it
// from its time.
func TestTimelog_AppendAtProperty(t *testing.T) {
	var zones []*time.Location
	for _, name := range []string{"UTC", "Europe/London", "America/New_York", "Asia/Kolkata", "Australia/Lord_Howe"} {
		zones = append(zones, mustZone(t, name))
	}
	words := []string{"work", "+htt", "@lunch", "#deep", DayEndEntry}
	r := rand.New(rand.NewPCG(20261118, 11))
	sortedDays := 0
	for range 1000 {
		g := generateDay(t, r, zones)
		repo := &recordingTimelogRepository{}
		l := NewTimelog(repo, g.log.Date)
		l.Entries = g.log.Entries

		// Entries can go anywhere from midnight to now, or the day's
		// end on a past day.
		limit := g.end.Add(-time.Second)
		if g.today {
			limit = g.now
		}
		seconds := int(limit.Sub(g.start)/time.Second) + 1
		inOrder := isInTimeOrder(t, l)
		if inOrder {
			sortedDays++
		}
		for k := range 1 + r.IntN(6) {
			at := g.start.Add(time.Duration(r.IntN(seconds)) * time.Second)
			if len(l.Entries) > 0 && r.IntN(4) == 0 { // at an entry's time
				ts := mustTimestamp(t, l.Entries[r.IntN(len(l.Entries))])
				if !ts.Before(g.start) && !ts.After(limit) {
					at = ts
				}
			}
			before := slices.Clone(l.Entries)
			entry := mustTask(t, "logged "+words[r.IntN(len(words))])
			if r.IntN(5) == 0 {
				entry = mustTask(t, DayEndEntry)
			}
			if err := l.AppendAt(entry, at, g.now); err != nil {
				t.Fatalf("day %v, insertion %d at %v (now %v): %v", g.start, k, at, g.now, err)
			}

			i := slices.Index(l.Entries, entry)
			if i < 0 || len(l.Entries) != len(before)+1 || !slices.Equal(slices.Delete(slices.Clone(l.Entries), i, i+1), before) {
				t.Fatalf("day %v: logging at %v turned\n%q into\n%q", g.start, at, raws(before), rawEntries(l))
			}
			if got, want := entry.Annotations[TimelogTimestampLabel], at.UTC().Format(time.RFC3339); got != want {
				t.Fatalf("day %v: logged at %v with ts:%s, want ts:%s", g.start, at, got, want)
			}
			if i > 0 && mustTimestamp(t, l.Entries[i-1]).After(at) {
				t.Fatalf("day %v: logged at %v after a later entry:\n%q", g.start, at, rawEntries(l))
			}
			for _, after := range l.Entries[i+1:] {
				if !mustTimestamp(t, after).After(at) {
					t.Fatalf("day %v: logged at %v before an entry at or before it:\n%q", g.start, at, rawEntries(l))
				}
			}
			if inOrder && !isInTimeOrder(t, l) {
				t.Fatalf("day %v: logging at %v put a day in order out of it:\n%q", g.start, at, rawEntries(l))
			}
			if saved := repo.saves[len(repo.saves)-1]; len(repo.saves) != k+1 || !slices.Equal(saved, rawEntries(l)) {
				t.Fatalf("day %v: %d saves, the last %q, want %d and %q", g.start, len(repo.saves), saved, k+1, rawEntries(l))
			}
			sheet, err := l.Timesheet(nil, g.now)
			if err != nil {
				t.Fatal(err)
			}
			if !IsDayEnd(entry) {
				runs := slices.IndexFunc(sheet.Intervals, func(iv Interval) bool { return iv.Entry == entry })
				if runs < 0 || !sheet.Intervals[runs].Start.Equal(at.Truncate(time.Second)) {
					t.Fatalf("day %v: logged at %v, the timesheet has %+v", g.start, at, sheet.Intervals)
				}
			}
		}
	}
	if sortedDays < 500 {
		t.Errorf("only %d of 1000 days were in time order to start with", sortedDays)
	}
}

func isInTimeOrder(t *testing.T, l *Timelog) bool {
	t.Helper()
	return slices.IsSortedFunc(l.Entries, func(a, b *Task) int {
		return mustTimestamp(t, a).Compare(mustTimestamp(t, b))
	})
}

func mustTimestamp(t *testing.T, entry *Task) time.Time {
	t.Helper()
	at, err := entryTimestamp(entry)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

func raws(entries []*Task) []string {
	return rawEntries(&Timelog{Entries: entries})
}
