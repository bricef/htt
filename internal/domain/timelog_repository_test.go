package domain_test

import (
	"errors"
	"testing"
	"time"
	_ "time/tzdata" // the zones below, wherever the tests run

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/storage"
)

func loadZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q): %v", name, err)
	}
	return loc
}

// saveDay saves date's log (YYYY-MM-DD, in loc) to r, with one entry,
// or none when empty.
func saveDay(t *testing.T, r domain.TimelogRepository, loc *time.Location, date string, empty bool) {
	t.Helper()
	day, err := time.ParseInLocation("2006-01-02 15:04", date+" 09:00", loc)
	if err != nil {
		t.Fatal(err)
	}
	l := &domain.Timelog{Date: day, Entries: []*domain.Task{}}
	if !empty {
		entry, err := domain.NewTask("work ts:" + day.UTC().Format(time.RFC3339))
		if err != nil {
			t.Fatal(err)
		}
		l.Entries = append(l.Entries, entry)
	}
	if err := r.Save(l); err != nil {
		t.Fatalf("Save %s: %v", date, err)
	}
}

func TestLastLogBefore(t *testing.T) {
	kolkata := loadZone(t, "Asia/Kolkata")
	santiago := loadZone(t, "America/Santiago")
	london := loadZone(t, "Europe/London")
	at := func(loc *time.Location, s string) time.Time {
		t.Helper()
		when, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
		if err != nil {
			t.Fatal(err)
		}
		return when
	}
	// 01:00 on 2026-11-19 in Kolkata is 19:30 on 2026-11-18 in UTC: the
	// days are the local zone's.
	night := at(kolkata, "2026-11-19 01:00")

	cases := []struct {
		name  string
		loc   *time.Location
		now   time.Time
		logs  []string // days with an entry
		empty []string // days whose log has no entries
		days  int
		want  string // the day found, "" for none
	}{
		{name: "nothing logged", loc: kolkata, now: night, days: 14},
		{name: "yesterday, in the local zone", loc: kolkata, now: night, logs: []string{"2026-11-18"}, days: 14, want: "2026-11-18"},
		{name: "today isn't before today", loc: kolkata, now: night, logs: []string{"2026-11-19"}, days: 14},
		{name: "nor is a later day", loc: kolkata, now: night, logs: []string{"2026-11-20"}, days: 14},
		{name: "the most recent day", loc: kolkata, now: night,
			logs: []string{"2026-11-12", "2026-11-17", "2026-11-15"}, days: 14, want: "2026-11-17"},
		{name: "after a gap", loc: kolkata, now: night, logs: []string{"2026-11-16"}, days: 14, want: "2026-11-16"},
		{name: "the furthest day it looks at", loc: kolkata, now: night, logs: []string{"2026-11-05"}, days: 14, want: "2026-11-05"},
		{name: "a day further back", loc: kolkata, now: night, logs: []string{"2026-11-04"}, days: 14},
		{name: "a log without entries isn't a log", loc: kolkata, now: night,
			logs: []string{"2026-11-16"}, empty: []string{"2026-11-18", "2026-11-17"}, days: 14, want: "2026-11-16"},
		{name: "no days to look at", loc: kolkata, now: night, logs: []string{"2026-11-18"}, days: 0},
		// Santiago's clocks go forward at midnight on 2026-09-06: that
		// day's midnight doesn't exist, and time.Date puts it at 23:00
		// on 2026-09-05.
		{name: "the day the clocks go forward at midnight", loc: santiago, now: at(santiago, "2026-09-07 10:00"),
			logs: []string{"2026-09-06", "2026-09-05"}, days: 1, want: "2026-09-06"},
		{name: "the day before it", loc: santiago, now: at(santiago, "2026-09-07 10:00"),
			logs: []string{"2026-09-05"}, days: 1},
		// A day back is a calendar day, not 24 hours. London's
		// 2026-03-29 lasts 23 hours: 24 hours before 00:30 the next day
		// is 23:30 on 2026-03-28. Its 2026-10-25 lasts 25: 48 hours
		// before 23:30 on 2026-10-26 is 00:30 on 2026-10-25.
		{name: "a 23-hour day", loc: london, now: at(london, "2026-03-30 00:30"),
			logs: []string{"2026-03-29"}, days: 1, want: "2026-03-29"},
		{name: "a 25-hour day", loc: london, now: at(london, "2026-10-26 23:30"),
			logs: []string{"2026-10-24"}, days: 2, want: "2026-10-24"},
	}
	repos := []struct {
		name string
		new  func(t *testing.T) domain.TimelogRepository
	}{
		{"memory", func(*testing.T) domain.TimelogRepository { return storage.NewMemoryTimelogRepository() }},
		{"file", func(t *testing.T) domain.TimelogRepository { return storage.NewFileTimelogRepository(t.TempDir()) }},
	}
	for _, repo := range repos {
		for _, c := range cases {
			t.Run(repo.name+": "+c.name, func(t *testing.T) {
				r := repo.new(t)
				for _, date := range c.logs {
					saveDay(t, r, c.loc, date, false)
				}
				for _, date := range c.empty {
					saveDay(t, r, c.loc, date, true)
				}
				got, err := domain.LastLogBefore(r, c.now, c.days)
				if err != nil {
					t.Fatalf("LastLogBefore: %v", err)
				}
				if got == nil {
					if c.want != "" {
						t.Errorf("LastLogBefore = nil, want %s's log", c.want)
					}
					return
				}
				if date := got.Date.In(c.loc).Format("2006-01-02"); date != c.want || c.want == "" {
					t.Errorf("LastLogBefore = %s's log, want %q", date, c.want)
				}
				if len(got.Entries) != 1 {
					t.Errorf("LastLogBefore: %d entries, want the day's one", len(got.Entries))
				}
			})
		}
	}
}

// failingDayRepository fails to load one day.
type failingDayRepository struct {
	domain.TimelogRepository
	day string
}

var errUnreadable = errors.New("unreadable")

func (r failingDayRepository) Day(date time.Time) (*domain.Timelog, error) {
	if date.Format("2006-01-02") == r.day {
		return nil, errUnreadable
	}
	return r.TimelogRepository.Day(date)
}

func TestLastLogBefore_ADayItCantLoadIsAnError(t *testing.T) {
	kolkata := loadZone(t, "Asia/Kolkata")
	now := time.Date(2026, 11, 19, 10, 0, 0, 0, kolkata)
	mem := storage.NewMemoryTimelogRepository()
	saveDay(t, mem, kolkata, "2026-11-16", false)
	r := failingDayRepository{mem, "2026-11-17"}
	got, err := domain.LastLogBefore(r, now, 14)
	if !errors.Is(err, errUnreadable) || got != nil {
		t.Fatalf("LastLogBefore = %v, %v; want nil and the error", got, err)
	}
	if want := "load the timelog for 2026-11-17: unreadable"; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
	// It stops at the most recent day with entries, and loads none
	// before it.
	r.day = "2026-11-15"
	if got, err := domain.LastLogBefore(r, now, 14); err != nil || got == nil {
		t.Errorf("LastLogBefore = %v, %v; want 2026-11-16's log", got, err)
	}
}
