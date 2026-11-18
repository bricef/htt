package dates

import (
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

func TestParseAt_AcceptedForms(t *testing.T) {
	kolkata := mustZone(t, "Asia/Kolkata")
	now := time.Date(2026, 11, 18, 15, 42, 30, 0, kolkata)
	at := func(y int, m time.Month, d, hh, mm int) time.Time {
		return time.Date(y, m, d, hh, mm, 0, 0, kolkata)
	}
	cases := []struct {
		input string
		want  time.Time
	}{
		{"09:30", at(2026, 11, 18, 9, 30)},
		{"9:30", at(2026, 11, 18, 9, 30)},
		{"00:00", at(2026, 11, 18, 0, 0)},
		{"0:00", at(2026, 11, 18, 0, 0)},
		{"23:59", at(2026, 11, 18, 23, 59)}, // later than now: parsed all the same
		{"15:42", at(2026, 11, 18, 15, 42)}, // now's minute, seconds zero
		{"  09:30\t", at(2026, 11, 18, 9, 30)},
		{"yesterday 17:30", at(2026, 11, 17, 17, 30)},
		{"yesterday 7:05", at(2026, 11, 17, 7, 5)},
		{"yesterday 00:00", at(2026, 11, 17, 0, 0)},
		{"yesterday 23:59", at(2026, 11, 17, 23, 59)},
		{"yesterday   17:30", at(2026, 11, 17, 17, 30)},
		{"2026-11-10 08:15", at(2026, 11, 10, 8, 15)},
		{"2026-11-10 8:15", at(2026, 11, 10, 8, 15)},
		{"2026-11-18 15:00", at(2026, 11, 18, 15, 0)},
		{"2026-11-20 09:00", at(2026, 11, 20, 9, 0)}, // the future too
		{"2024-02-29 12:00", at(2024, 2, 29, 12, 0)},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got, err := ParseAt(c.input, now)
			if err != nil {
				t.Fatalf("ParseAt(%q): %v", c.input, err)
			}
			if !got.Equal(c.want) || got.Location() != kolkata {
				t.Errorf("ParseAt(%q) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}

func TestParseAt_YesterdayIsTheCalendarDayBefore(t *testing.T) {
	// Yesterday is the date before now's, not 24 hours back: just after
	// midnight on the day after the clocks go forward, 24 hours back is
	// two dates ago.
	london := mustZone(t, "Europe/London")
	cases := []struct {
		now  time.Time
		want time.Time
	}{
		{time.Date(2026, 3, 30, 0, 30, 0, 0, london), time.Date(2026, 3, 29, 12, 0, 0, 0, london)},
		{time.Date(2026, 10, 26, 23, 30, 0, 0, london), time.Date(2026, 10, 25, 12, 0, 0, 0, london)},
		{time.Date(2026, 1, 1, 9, 0, 0, 0, london), time.Date(2025, 12, 31, 12, 0, 0, 0, london)},
		{time.Date(2026, 3, 1, 9, 0, 0, 0, london), time.Date(2026, 2, 28, 12, 0, 0, 0, london)},
	}
	for _, c := range cases {
		got, err := ParseAt("yesterday 12:00", c.now)
		if err != nil {
			t.Fatalf("ParseAt at %v: %v", c.now, err)
		}
		if !got.Equal(c.want) {
			t.Errorf("ParseAt(yesterday 12:00) at %v = %v, want %v", c.now, got, c.want)
		}
	}
}

func TestParseAt_RejectedForms(t *testing.T) {
	now := time.Date(2026, 11, 18, 15, 42, 30, 0, mustZone(t, "Asia/Kolkata"))
	for _, input := range []string{
		"", "   ",
		"25:00", "24:00", "09:60", "-1:30", "+9:30",
		"9.30", "930", "0930", "9:5", "9:", ":30", "009:30", "09:30:00", "9h30",
		"5pm", "9:30pm", "now", "noon", "lunch", "an hour ago",
		"2026-11-10", "yesterday", "today", "tomorrow",
		"today 09:30", "tomorrow 09:30", "Yesterday 17:30", "yesterday 9.30",
		"09:30 yesterday", "yesterday 17:30 extra", "17:30 2026-11-10",
		"2026-11-10T09:30", "2026-11-10 09:30 extra", "2026-11-10  09:30x",
		"2026-13-01 09:00", "2026-02-30 09:00", "2026-5-3 09:00", "26-11-10 09:00",
		"2026/11/10 09:00", "2026-11-10 24:00", "+026-11-10 09:00", "-026-11-10 09:00",
		"１２:３０", // full-width digits
	} {
		t.Run(input, func(t *testing.T) {
			got, err := ParseAt(input, now)
			if err == nil {
				t.Fatalf("ParseAt(%q) = %v, want an error", input, got)
			}
			// The error says which forms work.
			for _, form := range []string{"HH:MM", `"yesterday HH:MM"`, `"YYYY-MM-DD HH:MM"`, "24-hour"} {
				if !strings.Contains(err.Error(), form) {
					t.Errorf("ParseAt(%q) error %q doesn't mention %s", input, err, form)
				}
			}
			if !strings.Contains(err.Error(), `"`+input+`"`) {
				t.Errorf("ParseAt(%q) error %q doesn't quote the input", input, err)
			}
		})
	}
}

// On a day the clocks change, a time that doesn't exist or happens
// twice follows time.Date, which doesn't promise which side of the
// change it picks. These pin what it does, so a change in Go shows.
func TestParseAt_TheDayTheClocksChangeFollowsTimeDate(t *testing.T) {
	cases := []struct {
		zone, input string
		want        string // the time ParseAt gives, in UTC
		local       string // and on the local clock
	}{
		// London, clocks forward at 01:00 GMT: 01:30 doesn't exist and
		// lands an hour later, at 02:30 BST.
		{"Europe/London", "2026-03-29 01:30", "2026-03-29T01:30:00Z", "2026-03-29 02:30 BST"},
		{"Europe/London", "2026-03-29 02:30", "2026-03-29T01:30:00Z", "2026-03-29 02:30 BST"},
		// London, clocks back at 02:00 BST: 01:30 happens twice, and is
		// the second, in GMT.
		{"Europe/London", "2026-10-25 01:30", "2026-10-25T01:30:00Z", "2026-10-25 01:30 GMT"},
		// New York, clocks forward at 02:00 EST: 02:30 doesn't exist and
		// lands an hour earlier, at 01:30 EST.
		{"America/New_York", "2026-03-08 02:30", "2026-03-08T06:30:00Z", "2026-03-08 01:30 EST"},
		// New York, clocks back at 02:00 EDT: 01:30 is the first, in EDT.
		{"America/New_York", "2026-11-01 01:30", "2026-11-01T05:30:00Z", "2026-11-01 01:30 EDT"},
		// Santiago, clocks forward at midnight: 00:30 doesn't exist and
		// lands on the day before, at 23:30.
		{"America/Santiago", "2026-09-06 00:30", "2026-09-06T03:30:00Z", "2026-09-05 23:30 -04"},
		// Lord Howe moves its clocks by half an hour.
		{"Australia/Lord_Howe", "2026-10-04 02:15", "2026-10-03T15:45:00Z", "2026-10-04 02:45 +11"},
	}
	for _, c := range cases {
		t.Run(c.zone+" "+c.input, func(t *testing.T) {
			loc := mustZone(t, c.zone)
			now := time.Date(2026, 11, 18, 12, 0, 0, 0, loc)
			got, err := ParseAt(c.input, now)
			if err != nil {
				t.Fatalf("ParseAt(%q): %v", c.input, err)
			}
			if s := got.UTC().Format(time.RFC3339); s != c.want {
				t.Errorf("ParseAt(%q) in %s = %s UTC, want %s", c.input, c.zone, s, c.want)
			}
			if s := got.Format("2006-01-02 15:04 MST"); s != c.local {
				t.Errorf("ParseAt(%q) in %s = %s, want %s", c.input, c.zone, s, c.local)
			}
		})
	}

	// A time that exists on such a day is as typed, whichever form
	// names the day: in Santiago, the day has no midnight, but it has a
	// noon, and it is still yesterday half an hour into the next day
	// (going back a day from 00:30 would land in the gap, on the 5th).
	for _, c := range []struct {
		zone, input string
		now         time.Time
		want        string
	}{
		{"Europe/London", "yesterday 09:00", time.Date(2026, 3, 30, 8, 0, 0, 0, time.UTC), "2026-03-29 09:00 BST"},
		{"America/Santiago", "2026-09-06 12:00", time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC), "2026-09-06 12:00 -03"},
		{"America/Santiago", "yesterday 12:00", time.Date(2026, 9, 7, 3, 30, 0, 0, time.UTC), "2026-09-06 12:00 -03"},
	} {
		now := c.now.In(mustZone(t, c.zone))
		got, err := ParseAt(c.input, now)
		if err != nil {
			t.Fatalf("ParseAt(%q) at %v: %v", c.input, now, err)
		}
		if s := got.Format("2006-01-02 15:04 MST"); s != c.want {
			t.Errorf("ParseAt(%q) at %v = %s, want %s", c.input, now, s, c.want)
		}
	}
}
