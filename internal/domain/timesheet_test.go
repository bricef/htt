package domain

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // the zones below, wherever the tests run
)

func mustZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q): %v", name, err)
	}
	return loc
}

// localTime reads "2006-01-02 15:04" in loc.
func localTime(t *testing.T, loc *time.Location, s string) time.Time {
	t.Helper()
	at, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return at
}

// dayLog builds the Timelog for date (YYYY-MM-DD) from lines "HH:MM
// text", each an entry at that local time in loc, stamped in UTC as
// Append does. A line "YYYY-MM-DD HH:MM text" is an entry on another
// date, as a hand-edited file could hold.
func dayLog(t *testing.T, loc *time.Location, date string, lines ...string) *Timelog {
	t.Helper()
	l := &Timelog{Date: localTime(t, loc, date+" 00:00")}
	for _, line := range lines {
		when, text, _ := strings.Cut(line, " ")
		if strings.Count(when, "-") == 2 { // a date first
			clock, rest, _ := strings.Cut(text, " ")
			when, text = when+" "+clock, rest
		} else {
			when = date + " " + when
		}
		l.Entries = append(l.Entries, timestampedEntry(t, text, localTime(t, loc, when).UTC()))
	}
	return l
}

// describe writes an interval as "HH:MM-HH:MM category: entry" in the
// sheet's zone, with 24:00 for the end of the day; an interval no
// category matched is "uncategorised (default): entry".
func describe(s *Timesheet, iv Interval) string {
	clock := func(at time.Time) string {
		if at.Equal(s.End) {
			return "24:00"
		}
		return at.In(s.Start.Location()).Format("15:04")
	}
	text, _, _ := strings.Cut(iv.Entry.Raw, " "+TimelogTimestampLabel+":")
	category := iv.Category.Name
	if iv.Uncategorised {
		category = "uncategorised (" + category + ")"
	}
	return fmt.Sprintf("%s-%s %s: %s", clock(iv.Start), clock(iv.End), category, text)
}

func TestTimesheet_Rules(t *testing.T) {
	utc := time.UTC
	london := mustZone(t, "Europe/London")
	kolkata := mustZone(t, "Asia/Kolkata")
	cats := testCategories()

	cases := []struct {
		name    string
		loc     *time.Location
		date    string
		entries []string
		now     string // "" is three days later: date is a past day
		want    []string
		worked  time.Duration
		byCat   map[string]time.Duration // the time each category's tags matched
		uncat   time.Duration            // the time no category matched
		status  DayStatus
		endedAt string // HH:MM, for DayEnded
	}{
		{
			name: "the headline day: a break is an entry, @end ends the day",
			loc:  utc, date: "2026-11-02",
			entries: []string{"09:00 @start", "09:00 work +htt", "12:00 @lunch", "13:00 work +htt", "17:00 @end"},
			want: []string{
				"09:00-09:00 uncategorised (working): @start",
				"09:00-12:00 htt: work +htt",
				"12:00-13:00 break: @lunch",
				"13:00-17:00 htt: work +htt",
			},
			worked: 7 * time.Hour,
			byCat:  map[string]time.Duration{"htt": 7 * time.Hour, "break": time.Hour},
			status: DayEnded, endedAt: "17:00",
		},
		{
			name: "the same day with @end for lunch: the gap after @end counts as nothing",
			loc:  utc, date: "2026-11-02",
			entries: []string{"09:00 @start", "09:00 work", "12:00 @end", "13:00 work", "17:00 @end"},
			want: []string{
				"09:00-09:00 uncategorised (working): @start",
				"09:00-12:00 uncategorised (working): work",
				"13:00-17:00 uncategorised (working): work",
			},
			worked: 7 * time.Hour,
			byCat:  map[string]time.Duration{},
			uncat:  7 * time.Hour,
			status: DayEnded, endedAt: "17:00",
		},
		{
			name: "@start is an ordinary entry, in the default category",
			loc:  utc, date: "2026-11-02",
			entries: []string{"08:30 @start", "09:00 mail @email", "10:00 @end"},
			want:    []string{"08:30-09:00 uncategorised (working): @start", "09:00-10:00 admin: mail @email"},
			worked:  90 * time.Minute,
			byCat:   map[string]time.Duration{"admin": time.Hour},
			uncat:   30 * time.Minute,
			status:  DayEnded, endedAt: "10:00",
		},
		{
			name: "a past day without @end runs to 24:00 and is unended",
			loc:  utc, date: "2026-11-02",
			entries: []string{"09:00 work", "12:00 @lunch", "13:00 work"},
			want: []string{
				"09:00-12:00 uncategorised (working): work",
				"12:00-13:00 break: @lunch",
				"13:00-24:00 uncategorised (working): work",
			},
			worked: 14 * time.Hour,
			byCat:  map[string]time.Duration{"break": time.Hour},
			uncat:  14 * time.Hour,
			status: DayUnended,
		},
		{
			name: "an @end mid-day doesn't stop the day being unended",
			loc:  utc, date: "2026-11-02",
			entries: []string{"09:00 work", "12:00 @end", "23:00 late +htt"},
			want:    []string{"09:00-12:00 uncategorised (working): work", "23:00-24:00 htt: late +htt"},
			worked:  4 * time.Hour,
			byCat:   map[string]time.Duration{"htt": time.Hour},
			uncat:   3 * time.Hour,
			status:  DayUnended,
		},
		{
			name: "today's open entry runs until now, in progress",
			loc:  utc, date: "2026-11-02", now: "2026-11-02 11:30",
			entries: []string{"09:00 @start", "09:15 work +htt"},
			want:    []string{"09:00-09:15 uncategorised (working): @start", "09:15-11:30 htt: work +htt"},
			worked:  150 * time.Minute,
			byCat:   map[string]time.Duration{"htt": 135 * time.Minute},
			uncat:   15 * time.Minute,
			status:  DayInProgress,
		},
		{
			name: "today on a break: in progress, not worked",
			loc:  utc, date: "2026-11-02", now: "2026-11-02 12:20",
			entries: []string{"09:00 work", "12:00 @lunch"},
			want:    []string{"09:00-12:00 uncategorised (working): work", "12:00-12:20 break: @lunch"},
			worked:  3 * time.Hour,
			byCat:   map[string]time.Duration{"break": 20 * time.Minute},
			uncat:   3 * time.Hour,
			status:  DayInProgress,
		},
		{
			name: "today after @end: ended, nothing runs",
			loc:  utc, date: "2026-11-02", now: "2026-11-02 18:00",
			entries: []string{"09:00 work", "17:00 @end"},
			want:    []string{"09:00-17:00 uncategorised (working): work"},
			worked:  8 * time.Hour,
			byCat:   map[string]time.Duration{},
			uncat:   8 * time.Hour,
			status:  DayEnded, endedAt: "17:00",
		},
		{
			name: "today, back after an @end: in progress again",
			loc:  utc, date: "2026-11-02", now: "2026-11-02 14:00",
			entries: []string{"09:00 work", "12:00 @end", "13:00 work"},
			want:    []string{"09:00-12:00 uncategorised (working): work", "13:00-14:00 uncategorised (working): work"},
			worked:  4 * time.Hour,
			byCat:   map[string]time.Duration{},
			uncat:   4 * time.Hour,
			status:  DayInProgress,
		},
		{
			name: "only an @end: no intervals, ended",
			loc:  utc, date: "2026-11-02",
			entries: []string{"17:00 @end"},
			byCat:   map[string]time.Duration{},
			status:  DayEnded, endedAt: "17:00",
		},
		{
			name: "no entries",
			loc:  utc, date: "2026-11-02",
			byCat:  map[string]time.Duration{},
			status: DayEmpty,
		},
		{
			name: "only an entry that is just @end ends the day",
			loc:  utc, date: "2026-11-02",
			entries: []string{"09:00 fix the @end handling +htt", "10:00 @end of the sprint", "11:00 @end"},
			want:    []string{"09:00-10:00 htt: fix the @end handling +htt", "10:00-11:00 uncategorised (working): @end of the sprint"},
			worked:  2 * time.Hour,
			byCat:   map[string]time.Duration{"htt": time.Hour},
			uncat:   time.Hour,
			status:  DayEnded, endedAt: "11:00",
		},
		{
			name: "the day ends at local midnight, not UTC's",
			loc:  kolkata, date: "2026-11-02",
			entries: []string{"20:00 late +htt"}, // 14:30 UTC
			want:    []string{"20:00-24:00 htt: late +htt"},
			worked:  4 * time.Hour,
			byCat:   map[string]time.Duration{"htt": 4 * time.Hour},
			status:  DayUnended,
		},
		{
			name: "clocks go forward: the day has 23 hours",
			loc:  london, date: "2026-03-29",
			entries: []string{"00:00 all day"},
			want:    []string{"00:00-24:00 uncategorised (working): all day"},
			worked:  23 * time.Hour,
			byCat:   map[string]time.Duration{},
			uncat:   23 * time.Hour,
			status:  DayUnended,
		},
		{
			name: "clocks go back: the day has 25 hours",
			loc:  london, date: "2026-10-25",
			entries: []string{"00:00 all day"},
			want:    []string{"00:00-24:00 uncategorised (working): all day"},
			worked:  25 * time.Hour,
			byCat:   map[string]time.Duration{},
			uncat:   25 * time.Hour,
			status:  DayUnended,
		},
		{
			name: "entries out of order in the file are read in time order",
			loc:  utc, date: "2026-11-02",
			entries: []string{"13:00 work", "09:00 work +htt", "17:00 @end", "12:00 @lunch"},
			want: []string{
				"09:00-12:00 htt: work +htt",
				"12:00-13:00 break: @lunch",
				"13:00-17:00 uncategorised (working): work",
			},
			worked: 7 * time.Hour,
			byCat:  map[string]time.Duration{"htt": 3 * time.Hour, "break": time.Hour},
			uncat:  4 * time.Hour,
			status: DayEnded, endedAt: "17:00",
		},
		{
			name: "an entry from before the day counts from midnight; one after it, not at all",
			loc:  utc, date: "2026-11-02",
			entries: []string{"2026-11-01 23:00 night +htt", "01:00 @end", "09:00 work", "2026-11-03 01:00 tomorrow"},
			want: []string{
				"00:00-01:00 htt: night +htt",
				"09:00-24:00 uncategorised (working): work",
				"24:00-24:00 uncategorised (working): tomorrow",
			},
			worked: 16 * time.Hour,
			byCat:  map[string]time.Duration{"htt": time.Hour},
			uncat:  15 * time.Hour,
			status: DayUnended,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := dayLog(t, tc.loc, tc.date, tc.entries...)
			now := localTime(t, tc.loc, tc.date+" 12:00").AddDate(0, 0, 3)
			if tc.now != "" {
				now = localTime(t, tc.loc, tc.now)
			}
			s, err := l.Timesheet(mustCategoriser(t, cats), now)
			if err != nil {
				t.Fatalf("Timesheet: %v", err)
			}
			var got []string
			for _, iv := range s.Intervals {
				got = append(got, describe(s, iv))
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("intervals:\n got: %q\nwant: %q", got, tc.want)
			}
			if s.Worked() != tc.worked {
				t.Errorf("Worked() = %v, want %v", s.Worked(), tc.worked)
			}
			if byCat := s.ByCategory(); !maps.Equal(byCat, tc.byCat) {
				t.Errorf("ByCategory() = %v, want %v", byCat, tc.byCat)
			}
			if s.Uncategorised() != tc.uncat {
				t.Errorf("Uncategorised() = %v, want %v", s.Uncategorised(), tc.uncat)
			}
			if s.Status != tc.status {
				t.Errorf("Status = %v, want %v", s.Status, tc.status)
			}
			endedAt := ""
			if !s.EndedAt.IsZero() {
				endedAt = s.EndedAt.In(tc.loc).Format("15:04")
			}
			if endedAt != tc.endedAt {
				t.Errorf("EndedAt = %q, want %q", endedAt, tc.endedAt)
			}
			current := s.Current()
			if tc.status == DayInProgress {
				if current == nil || current != &s.Intervals[len(s.Intervals)-1] || !current.End.Equal(now) {
					t.Errorf("Current() = %v, want the last interval, running until now", current)
				}
			} else if current != nil {
				t.Errorf("Current() = %v, want nil: nothing is in progress", describe(s, *current))
			}
		})
	}
}

func TestTimesheet_DayBoundsAreLocal(t *testing.T) {
	kolkata := mustZone(t, "Asia/Kolkata")
	l := dayLog(t, kolkata, "2026-11-02")
	s, err := l.Timesheet(nil, localTime(t, kolkata, "2026-11-10 09:00"))
	if err != nil {
		t.Fatal(err)
	}
	if want := localTime(t, kolkata, "2026-11-02 00:00"); !s.Start.Equal(want) {
		t.Errorf("Start = %v, want %v", s.Start, want)
	}
	if want := localTime(t, kolkata, "2026-11-03 00:00"); !s.End.Equal(want) {
		t.Errorf("End = %v, want %v", s.End, want)
	}
}

func TestTimesheet_TheDateIsTheLogsOwn(t *testing.T) {
	// The report asks for days as dates in whatever zone it built them
	// in; the sheet takes the log's year, month and day, and reads them
	// in now's zone.
	kolkata := mustZone(t, "Asia/Kolkata")
	l := &Timelog{Date: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)}
	l.Entries = append(l.Entries, timestampedEntry(t, "late", localTime(t, kolkata, "2026-11-02 23:00")))
	s, err := l.Timesheet(nil, localTime(t, kolkata, "2026-11-10 09:00"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Worked() != time.Hour {
		t.Errorf("Worked() = %v, want 1h: from 23:00 to midnight in Kolkata", s.Worked())
	}
}

func TestTimesheet_NilCategoriserPutsEverythingInWorking(t *testing.T) {
	// With no categories nothing matches: every interval is
	// uncategorised, in the built-in working, and counts as work.
	l := dayLog(t, time.UTC, "2026-11-02", "09:00 work", "12:00 @lunch", "13:00 +htt", "17:00 @end")
	s, err := l.Timesheet(nil, localTime(t, time.UTC, "2026-11-05 09:00"))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.ByCategory(); len(got) != 0 {
		t.Errorf("ByCategory() = %v, want nothing: no category matched", got)
	}
	if s.Uncategorised() != 8*time.Hour || s.Worked() != 8*time.Hour {
		t.Errorf("Uncategorised() = %v, Worked() = %v, want 8h each", s.Uncategorised(), s.Worked())
	}
	for _, iv := range s.Intervals {
		if !iv.Uncategorised || iv.Category.Name != DefaultCategoryName || !iv.Category.Work {
			t.Errorf("interval %s: want uncategorised, in working, counting as work", describe(s, iv))
		}
	}
}

func TestTimesheet_UncategorisedTimeFollowsTheDefault(t *testing.T) {
	// The same day under each kind of default category. Time no
	// category matched is uncategorised, never in a category's own
	// time, and counts as worked when the default counts as work. The
	// default's own time is only what its tags matched.
	day := []string{"09:00 review +htt", "11:00 filing @desk", "12:00 @lunch", "13:00 reading the news", "16:00 @end"}
	breakAndHtt := []Category{
		{Name: "break", Tags: []string{"@lunch"}, Work: false},
		{Name: "htt", Tags: []string{"+htt"}, Work: true},
	}
	withWorking := func(work bool) []Category {
		return append(slices.Clone(breakAndHtt), Category{Name: "working", Tags: []string{"@desk"}, Work: work})
	}
	withDefault := func(categories []Category, name string) *Categoriser {
		c, err := mustCategoriser(t, categories).WithDefault(name)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}

	cases := []struct {
		name   string
		c      *Categoriser
		want   []string
		byCat  map[string]time.Duration
		uncat  time.Duration
		worked time.Duration
	}{
		{
			name: "the built-in working: uncategorised time is worked",
			c:    mustCategoriser(t, breakAndHtt),
			want: []string{
				"09:00-11:00 htt: review +htt",
				"11:00-12:00 uncategorised (working): filing @desk",
				"12:00-13:00 break: @lunch",
				"13:00-16:00 uncategorised (working): reading the news",
			},
			byCat: map[string]time.Duration{"htt": 2 * time.Hour, "break": time.Hour},
			uncat: 4 * time.Hour, worked: 6 * time.Hour,
		},
		{
			name: "working defined with tags: its line holds its matches only",
			c:    mustCategoriser(t, withWorking(true)),
			want: []string{
				"09:00-11:00 htt: review +htt",
				"11:00-12:00 working: filing @desk",
				"12:00-13:00 break: @lunch",
				"13:00-16:00 uncategorised (working): reading the news",
			},
			byCat: map[string]time.Duration{"htt": 2 * time.Hour, "working": time.Hour, "break": time.Hour},
			uncat: 3 * time.Hour, worked: 6 * time.Hour,
		},
		{
			name: "working defined with tags and work: false: neither its time nor uncategorised time is worked",
			c:    mustCategoriser(t, withWorking(false)),
			want: []string{
				"09:00-11:00 htt: review +htt",
				"11:00-12:00 working: filing @desk",
				"12:00-13:00 break: @lunch",
				"13:00-16:00 uncategorised (working): reading the news",
			},
			byCat: map[string]time.Duration{"htt": 2 * time.Hour, "working": time.Hour, "break": time.Hour},
			uncat: 3 * time.Hour, worked: 2 * time.Hour,
		},
		{
			name: "a defined category that isn't work: uncategorised time isn't worked",
			c:    withDefault(breakAndHtt, "break"),
			want: []string{
				"09:00-11:00 htt: review +htt",
				"11:00-12:00 uncategorised (break): filing @desk",
				"12:00-13:00 break: @lunch",
				"13:00-16:00 uncategorised (break): reading the news",
			},
			byCat: map[string]time.Duration{"htt": 2 * time.Hour, "break": time.Hour},
			uncat: 4 * time.Hour, worked: 2 * time.Hour,
		},
		{
			name: "a tagless default has no time of its own",
			c:    withDefault(append(slices.Clone(breakAndHtt), Category{Name: "idle", Work: false}), "idle"),
			want: []string{
				"09:00-11:00 htt: review +htt",
				"11:00-12:00 uncategorised (idle): filing @desk",
				"12:00-13:00 break: @lunch",
				"13:00-16:00 uncategorised (idle): reading the news",
			},
			byCat: map[string]time.Duration{"htt": 2 * time.Hour, "break": time.Hour},
			uncat: 4 * time.Hour, worked: 2 * time.Hour,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := dayLog(t, time.UTC, "2026-11-02", day...).Timesheet(tc.c, localTime(t, time.UTC, "2026-11-05 09:00"))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, iv := range s.Intervals {
				got = append(got, describe(s, iv))
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("intervals:\n got: %q\nwant: %q", got, tc.want)
			}
			if byCat := s.ByCategory(); !maps.Equal(byCat, tc.byCat) {
				t.Errorf("ByCategory() = %v, want %v", byCat, tc.byCat)
			}
			if s.Uncategorised() != tc.uncat {
				t.Errorf("Uncategorised() = %v, want %v", s.Uncategorised(), tc.uncat)
			}
			if s.Worked() != tc.worked {
				t.Errorf("Worked() = %v, want %v", s.Worked(), tc.worked)
			}
		})
	}
}

func TestTimesheet_ErrorOnMissingOrMalformedTimestamp(t *testing.T) {
	bad := mustTask(t, "garbage ts")
	if _, err := bad.Annotate(TimelogTimestampLabel, "definitely-not-rfc3339"); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []*Task{mustTask(t, "missing ts"), bad} {
		l := &Timelog{
			Date:    time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
			Entries: []*Task{timestampedEntry(t, "ok", time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)), entry},
		}
		_, err := l.Timesheet(nil, time.Now())
		if err == nil {
			t.Errorf("Timesheet with %q: want an error", entry.Raw)
			continue
		}
		if !strings.Contains(err.Error(), "entry 1") || !strings.Contains(err.Error(), TimelogTimestampLabel) {
			t.Errorf("error = %q, want it to name entry 1 and the %s annotation", err, TimelogTimestampLabel)
		}
	}
}

func TestIsDayEnd(t *testing.T) {
	for raw, want := range map[string]bool{
		"@end":                           true,
		"@end ts:2026-11-02T17:00:00Z":   true,
		"(A) @end":                       true, // a priority isn't text
		"@end of the sprint":             false,
		"fix @end handling":              false,
		"@END":                           false,
		"@ending":                        false,
		"@start":                         false,
		"2026-11-02 @end":                true, // a created-on date isn't text
		"@end note:hand-added":           true,
		"@end ts:2026-11-02T17:00:00Z x": false,
	} {
		if got := IsDayEnd(mustTask(t, raw)); got != want {
			t.Errorf("IsDayEnd(%q) = %v, want %v", raw, got, want)
		}
	}
}
