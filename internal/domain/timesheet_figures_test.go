package domain_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/storage"
)

// figuresCategories are the categories the figures tests read time in:
// a break that isn't work, two work categories, and the built-in
// default, working, which has no tags.
func figuresCategories(t *testing.T) *domain.Categoriser {
	t.Helper()
	c, err := domain.NewCategoriser([]domain.Category{
		{Name: "break", Tags: []string{"@lunch"}, Work: false},
		{Name: "htt", Tags: []string{"+htt"}, Work: true},
		{Name: "admin", Tags: []string{"@email"}, Work: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// figuresRepo is an in-memory repository holding logs: lines "HH:MM
// text" by date (YYYY-MM-DD), each an entry at that time in loc.
func figuresRepo(t *testing.T, loc *time.Location, logs map[string][]string) *storage.MemoryTimelogRepository {
	t.Helper()
	r := storage.NewMemoryTimelogRepository()
	for date, lines := range logs {
		day, err := time.ParseInLocation("2006-01-02 15:04", date+" 12:00", loc)
		if err != nil {
			t.Fatal(err)
		}
		l := &domain.Timelog{Date: day}
		for _, line := range lines {
			clock, text, _ := strings.Cut(line, " ")
			at, err := time.ParseInLocation("2006-01-02 15:04", date+" "+clock, loc)
			if err != nil {
				t.Fatal(err)
			}
			entry, err := domain.NewTask(text + " ts:" + at.UTC().Format(time.RFC3339))
			if err != nil {
				t.Fatal(err)
			}
			l.Entries = append(l.Entries, entry)
		}
		if err := r.Save(l); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

// countingRepository counts the days it's asked for, by date.
type countingRepository struct {
	domain.TimelogRepository
	asked map[string]int
}

func (r countingRepository) Day(date time.Time) (*domain.Timelog, error) {
	r.asked[date.Format("2006-01-02")]++
	return r.TimelogRepository.Day(date)
}

func at(t *testing.T, loc *time.Location, s string) time.Time {
	t.Helper()
	when, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
	if err != nil {
		t.Fatal(err)
	}
	return when
}

func ymd(days []*domain.Timesheet) string {
	var out []string
	for _, d := range days {
		out = append(out, d.Date.Format("2006-01-02"))
	}
	return strings.Join(out, " ")
}

func TestCalendarDate_ADayOnTheCalendarAtNoon(t *testing.T) {
	// In Asunción the clocks went forward at midnight on Sunday 1
	// October 2023: that midnight never happened, and Go puts a time
	// that doesn't exist on the day before. The date is still 1 October.
	asuncion := loadZone(t, "America/Asuncion")
	kolkata := loadZone(t, "Asia/Kolkata")
	cases := []struct {
		layout, s string
		loc       *time.Location
		want      string
	}{
		{"2006-01-02", "2023-10-01", asuncion, "2023-10-01"},
		{"2006-01-02", "2026-11-19", kolkata, "2026-11-19"},
		{"2006-01", "2023-10", asuncion, "2023-10-01"},
		{"2006-01", "2026-02", kolkata, "2026-02-01"},
	}
	for _, c := range cases {
		got, err := domain.CalendarDate(c.layout, c.s, c.loc)
		if err != nil {
			t.Errorf("CalendarDate(%q, %q): %v", c.layout, c.s, err)
			continue
		}
		if f := got.Format("2006-01-02 15:04:05.999"); f != c.want+" 12:00:00" || got.Location() != c.loc {
			t.Errorf("CalendarDate(%q, %q) = %s in %v, want %s at noon in %v", c.layout, c.s, f, got.Location(), c.want, c.loc)
		}
	}
	for _, c := range []struct{ layout, s string }{
		{"2006-01-02", "2026-13-01"}, {"2006-01-02", "2026-11-31"}, {"2006-01-02", "2026-11-1"},
		{"2006-01-02", "yesterday"}, {"2006-01-02", ""}, {"2006-01", "2026-1"}, {"2006-01", "2026-10-01"},
	} {
		if got, err := domain.CalendarDate(c.layout, c.s, kolkata); err == nil {
			t.Errorf("CalendarDate(%q, %q) = %v, want an error", c.layout, c.s, got)
		}
	}
}

func TestReadDays_TodayAndBeforeFromTheirLogs(t *testing.T) {
	kolkata := loadZone(t, "Asia/Kolkata")
	now := at(t, kolkata, "2026-11-19 15:20")
	mem := figuresRepo(t, kolkata, map[string][]string{
		"2026-11-17": {"09:00 fix +htt", "17:00 @end"},
		"2026-11-18": {"09:00 mail @email"},              // no @end: to 24:00
		"2026-11-19": {"09:00 fix +htt"},                 // today: until now
		"2026-11-20": {"09:00 written by hand for +htt"}, // to come: not read
	})
	r := countingRepository{mem, map[string]int{}}
	dates := domain.Period{First: at(t, kolkata, "2026-11-17 12:00"), Last: at(t, kolkata, "2026-11-21 12:00")}.Dates()
	days, err := domain.ReadDays(r, figuresCategories(t), dates, now)
	if err != nil {
		t.Fatal(err)
	}
	if ymd(days) != "2026-11-17 2026-11-18 2026-11-19 2026-11-20 2026-11-21" {
		t.Fatalf("days %s", ymd(days))
	}
	want := []struct {
		status domain.DayStatus
		worked time.Duration
	}{
		{domain.DayEnded, 8 * time.Hour},
		{domain.DayUnended, 15 * time.Hour},
		{domain.DayInProgress, 6*time.Hour + 20*time.Minute},
		{domain.DayEmpty, 0},
		{domain.DayEmpty, 0},
	}
	for i, w := range want {
		if days[i].Status != w.status || days[i].Worked() != w.worked {
			t.Errorf("%s: %v, %v worked; want %v, %v", days[i].Date.Format("2006-01-02"), days[i].Status, days[i].Worked(), w.status, w.worked)
		}
	}
	if fmt.Sprint(r.asked) != "map[2026-11-17:1 2026-11-18:1 2026-11-19:1]" {
		t.Errorf("logs read: %v, want today's and before, once each", r.asked)
	}
	if days[0].Intervals[0].Category.Name != "htt" {
		t.Errorf("the 17th's entry in %q, want htt", days[0].Intervals[0].Category.Name)
	}

	// A log it can't load, or read, is an error naming the day.
	if _, err := domain.ReadDays(failingDayRepository{mem, "2026-11-18"}, nil, dates, now); !errors.Is(err, errUnreadable) ||
		err.Error() != "load the timelog for 2026-11-18: unreadable" {
		t.Errorf("a day it can't load: %v", err)
	}
	bad := &domain.Timelog{Date: at(t, kolkata, "2026-11-17 12:00")}
	entry, _ := domain.NewTask("an entry without its time")
	bad.Entries = append(bad.Entries, entry)
	if err := mem.Save(bad); err != nil {
		t.Fatal(err)
	}
	if _, err := domain.ReadDays(mem, nil, dates, now); err == nil || !strings.HasPrefix(err.Error(), "read the timelog for 2026-11-17: ") {
		t.Errorf("a day it can't read: %v", err)
	}
}

func TestSoFar_TheDaysUpToToday(t *testing.T) {
	kolkata := loadZone(t, "Asia/Kolkata")
	week := domain.WeekOf(at(t, kolkata, "2026-11-19 12:00"))
	days, err := domain.ReadDays(figuresRepo(t, kolkata, nil), nil, week.Dates(), at(t, kolkata, "2026-11-19 12:00"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ now, want string }{
		{"2026-11-19 00:00", "2026-11-16 2026-11-17 2026-11-18 2026-11-19"}, // today from its first minute
		{"2026-11-19 23:59", "2026-11-16 2026-11-17 2026-11-18 2026-11-19"},
		{"2026-11-16 00:30", "2026-11-16"},
		{"2026-11-22 18:00", "2026-11-16 2026-11-17 2026-11-18 2026-11-19 2026-11-20 2026-11-21 2026-11-22"},
		{"2026-11-23 00:00", "2026-11-16 2026-11-17 2026-11-18 2026-11-19 2026-11-20 2026-11-21 2026-11-22"}, // over
		{"2026-11-15 23:59", ""}, // to come
	}
	for _, c := range cases {
		if got := ymd(domain.SoFar(days, at(t, kolkata, c.now))); got != c.want {
			t.Errorf("SoFar at %s = %q, want %q", c.now, got, c.want)
		}
	}
}

func TestWeek_TheWeekSoFar(t *testing.T) {
	kolkata := loadZone(t, "Asia/Kolkata")
	c := figuresCategories(t)
	r := figuresRepo(t, kolkata, map[string][]string{
		"2026-11-09": {"09:00 fix +htt", "17:00 @end"},
		"2026-11-16": {"09:00 fix +htt", "12:00 @lunch", "13:00 read the news", "17:00 @end"},
		"2026-11-17": {"20:00 mail @email"}, // no @end
		"2026-11-19": {"09:00 fix +htt"},    // today, in progress
		"2026-11-21": {"09:00 by hand +htt", "17:00 @end"},
	})
	now := at(t, kolkata, "2026-11-19 15:20")
	h := time.Hour

	w, err := domain.Week(r, c, at(t, kolkata, "2026-11-22 12:00"), now) // the Sunday: its week
	if err != nil {
		t.Fatal(err)
	}
	if !w.Period.First.Equal(domain.WeekOf(now).First) || !w.Period.Last.Equal(domain.WeekOf(now).Last) || len(w.Days) != 7 || w.SoFar != 4 {
		t.Fatalf("the week of %s: %d days, %d so far", w.Period.First.Format("2006-01-02"), len(w.Days), w.SoFar)
	}
	if want := domain.Summarise(w.Days[:4]); fmt.Sprint(w.Summary) != fmt.Sprint(want) {
		t.Errorf("summary %+v, want over the 4 days so far: %+v", w.Summary, want)
	}
	if w.Summary.Days != 4 || w.Summary.WorkingDays != 3 || w.Summary.Totals.Worked != 7*h+4*h+6*h+20*time.Minute ||
		w.Summary.Totals.ToMidnight != 4*h || len(w.Summary.Unended) != 1 {
		t.Errorf("summary %+v", w.Summary)
	}
	if w.Days[5].Status != domain.DayEmpty || w.Days[5].Worked() != 0 {
		t.Errorf("Saturday, to come: %v, %v worked", w.Days[5].Status, w.Days[5].Worked())
	}

	past, err := domain.Week(r, c, at(t, kolkata, "2026-11-09 12:00"), now)
	if err != nil {
		t.Fatal(err)
	}
	if past.SoFar != 7 || past.Summary.Days != 7 || past.Summary.Totals.Worked != 8*h || past.Summary.AverageWeek != 8*h {
		t.Errorf("a week that's over: %d so far, %+v", past.SoFar, past.Summary)
	}
	next, err := domain.Week(r, c, at(t, kolkata, "2026-11-23 12:00"), now)
	if err != nil {
		t.Fatal(err)
	}
	if next.SoFar != 0 || next.Summary.Days != 0 || len(next.Days) != 7 {
		t.Errorf("a week to come: %d so far, %+v", next.SoFar, next.Summary)
	}
}

func TestMonth_TheMonthSoFar(t *testing.T) {
	// November 2026 starts on a Sunday; today is Thursday the 19th.
	kolkata := loadZone(t, "Asia/Kolkata")
	c := figuresCategories(t)
	r := figuresRepo(t, kolkata, map[string][]string{
		"2026-10-26": {"09:00 fix +htt", "17:00 @end"}, // within 6 days of the 1st: in its rolling week
		"2026-10-31": {"09:00 fix +htt", "10:00 @end"},
		"2026-11-02": {"09:00 fix +htt", "12:00 @lunch", "13:00 mail @email", "17:00 @end"},
		"2026-11-03": {"09:00 read the news", "11:00 @end"},
		"2026-11-16": {"09:00 fix +htt", "12:00 @end"},
		"2026-11-19": {"09:00 fix +htt"},                   // today, in progress
		"2026-11-24": {"09:00 by hand +htt", "17:00 @end"}, // to come
	})
	now := at(t, kolkata, "2026-11-19 15:20")
	h := time.Hour

	m, err := domain.Month(r, c, now, now)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Period.First.Equal(domain.MonthOf(now).First) || !m.Period.Last.Equal(domain.MonthOf(now).Last) ||
		len(m.Days) != 30 || m.SoFar != 19 || m.Summary.Days != 19 {
		t.Fatalf("November: %d days, %d so far, summary over %d", len(m.Days), m.SoFar, m.Summary.Days)
	}
	worked := 3*h + 4*h + 2*h + 3*h + 6*h + 20*time.Minute
	if m.Summary.Totals.Worked != worked || m.Summary.AverageWeek != worked*7/19 {
		t.Errorf("summary %+v", m.Summary)
	}
	var lengths []int
	for _, w := range m.Weeks {
		lengths = append(lengths, w.Days)
	}
	if fmt.Sprint(lengths) != "[1 7 7 7 7 1]" || m.Weeks[3].Totals.Worked != 3*h+6*h+20*time.Minute || m.Weeks[4].Totals.Worked != 0 {
		t.Errorf("weeks of %v days, the 16th's %v worked, the 23rd's %v", lengths, m.Weeks[3].Totals.Worked, m.Weeks[4].Totals.Worked)
	}
	if want := domain.WeekdayAverages(m.Days[:19]); fmt.Sprint(m.Weekdays) != fmt.Sprint(want) {
		t.Errorf("weekdays over the days so far:\n got %+v\nwant %+v", m.Weekdays, want)
	}
	if m.Weekdays[0].Days != 3 || m.Weekdays[1].Days != 3 || m.Weekdays[4].Days != 2 {
		t.Errorf("Mondays %d, Tuesdays %d, Fridays %d; want 3, 3 and 2 so far", m.Weekdays[0].Days, m.Weekdays[1].Days, m.Weekdays[4].Days)
	}
	if len(m.Rolling) != 19 || m.Rolling[0].Worked != 8*h+h || m.Rolling[1].Worked != h+3*h+4*h {
		t.Errorf("%d rolling totals, the first two %v and %v", len(m.Rolling), m.Rolling[0].Worked, m.Rolling[1].Worked)
	}
	if m.Halves[0].Days != 15 || m.Halves[1].Days != 4 || m.Halves[1].Totals.Worked != 3*h+6*h+20*time.Minute {
		t.Errorf("halves %+v", m.Halves)
	}
	if want := domain.CategoryTable(c, m.Summary.Totals, m.Halves[0], m.Halves[1]); fmt.Sprint(m.Table) != fmt.Sprint(want) {
		t.Errorf("table:\n got %+v\nwant %+v", m.Table, want)
	}

	oct, err := domain.Month(r, c, at(t, kolkata, "2026-10-01 12:00"), now)
	if err != nil {
		t.Fatal(err)
	}
	if oct.SoFar != 31 || oct.Summary.Days != 31 || len(oct.Rolling) != 31 || oct.Halves[1].Days != 16 {
		t.Errorf("October: %d so far, %d rolling totals, second half %d days", oct.SoFar, len(oct.Rolling), oct.Halves[1].Days)
	}
	dec, err := domain.Month(r, c, at(t, kolkata, "2026-12-01 12:00"), now)
	if err != nil {
		t.Fatal(err)
	}
	if dec.SoFar != 0 || len(dec.Rolling) != 0 || dec.Halves[0].Days != 0 || len(dec.Weeks) != 5 {
		t.Errorf("December, to come: %d so far, %d rolling totals, %d weeks", dec.SoFar, len(dec.Rolling), len(dec.Weeks))
	}
}

func TestCategoryTable(t *testing.T) {
	h := time.Hour
	total := domain.Totals{Worked: 10 * h, Breaks: 2 * h, Uncategorised: 2 * h,
		ByCategory: map[string]time.Duration{"break": 2 * h, "htt": 6 * h, "admin": 2 * h}}
	first := domain.Stretch{Days: 15, Totals: domain.Totals{Worked: 6 * h, Uncategorised: 2 * h,
		ByCategory: map[string]time.Duration{"htt": 3 * h, "break": 2 * h, "admin": h}}}
	second := domain.Stretch{Days: 14, Totals: domain.Totals{Worked: 4 * h,
		ByCategory: map[string]time.Duration{"htt": 3 * h, "admin": h}}}

	rows := domain.CategoryTable(figuresCategories(t), total, first, second)
	var names []string
	for _, r := range rows {
		names = append(names, r.Name)
	}
	if strings.Join(names, " ") != "break htt admin uncategorised" {
		t.Fatalf("rows %v: want the categories with tags, in order, then uncategorised", names)
	}
	brk, htt, admin, uncat := rows[0], rows[1], rows[2], rows[3]
	if brk.Work || brk.Total != 2*h || brk.HasShare || !brk.HasPerWeek[0] || brk.PerWeek[0] != 2*h*7/15 ||
		brk.PerWeek[1] != 0 || !brk.HasChange || brk.Change != -1 {
		t.Errorf("break: %+v", brk)
	}
	if !htt.Work || htt.Total != 6*h || !htt.HasShare || htt.Share != 0.6 ||
		htt.PerWeek != [2]time.Duration{3 * h * 7 / 15, 3 * h * 7 / 14} || htt.HasPerWeek != [2]bool{true, true} ||
		!htt.HasChange || htt.Change != float64(3*h*7/14)/float64(3*h*7/15)-1 {
		t.Errorf("htt: %+v", htt)
	}
	if admin.Share != 0.2 || admin.Uncategorised {
		t.Errorf("admin: %+v", admin)
	}
	// Uncategorised time is worked time, as the default (working) is:
	// it has a share; none in the second half is a change of -100%.
	if !uncat.Uncategorised || !uncat.Work || uncat.Total != 2*h || !uncat.HasShare || uncat.Share != 0.2 ||
		uncat.PerWeek[0] != 2*h*7/15 || uncat.Change != -1 {
		t.Errorf("uncategorised: %+v", uncat)
	}

	// The second half without days yet: no time per week, no change.
	// The first without time: no change.
	rows = domain.CategoryTable(figuresCategories(t), first.Totals, first, domain.Stretch{})
	if r := rows[1]; r.HasPerWeek[1] || r.PerWeek[1] != 0 || r.HasChange || !r.HasPerWeek[0] {
		t.Errorf("htt, without a second half: %+v", r)
	}
	rows = domain.CategoryTable(figuresCategories(t), second.Totals, domain.Stretch{Days: 15, Totals: domain.Totals{}}, second)
	if r := rows[1]; !r.HasPerWeek[0] || r.PerWeek[0] != 0 || r.HasChange {
		t.Errorf("htt, without time in the first half: %+v", r)
	}
	// Without worked time, nothing has a share.
	rows = domain.CategoryTable(figuresCategories(t), domain.Totals{Breaks: h, ByCategory: map[string]time.Duration{"break": h}}, first, second)
	for _, r := range rows {
		if r.HasShare {
			t.Errorf("%s has a share of no worked time", r.Name)
		}
	}

	// A default that isn't work: uncategorised time isn't worked time,
	// and has no share of it. A default with tags is a row of its own.
	idle, err := domain.NewCategoriser([]domain.Category{
		{Name: "htt", Tags: []string{"+htt"}, Work: true},
		{Name: "idle", Tags: []string{"@idle"}, Work: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	if idle, err = idle.WithDefault("idle"); err != nil {
		t.Fatal(err)
	}
	rows = domain.CategoryTable(idle, total, first, second)
	if len(rows) != 3 || rows[1].Name != "idle" || rows[2].Work || rows[2].HasShare || rows[2].Total != 2*h {
		t.Errorf("a default that isn't work: %+v", rows)
	}
	if rows := domain.CategoryTable(nil, total, first, second); len(rows) != 1 || !rows[0].Uncategorised || !rows[0].Work {
		t.Errorf("no categories: %+v", rows)
	}
}

func TestSummary_UncategorisedShare(t *testing.T) {
	h := time.Hour
	s := domain.Summary{Totals: domain.Totals{Worked: 8 * h, Uncategorised: 2 * h}}
	if share, ok := s.UncategorisedShare(domain.Category{Name: "working", Work: true}); !ok || share != 0.25 {
		t.Errorf("default working: %v, %v; want 0.25", share, ok)
	}
	if share, ok := s.UncategorisedShare(domain.Category{Name: "idle"}); ok {
		t.Errorf("a default that isn't work: %v, want none", share)
	}
	if share, ok := (domain.Summary{}).UncategorisedShare(domain.Category{Name: "working", Work: true}); ok {
		t.Errorf("no worked time: %v, want none", share)
	}
}

func TestStretch_PerWeek(t *testing.T) {
	h := time.Hour
	s := domain.Stretch{Days: 4, Totals: domain.Totals{Worked: 8 * h, Breaks: h, ByCategory: map[string]time.Duration{"htt": 8 * h}}}
	pw, ok := s.PerWeek()
	if !ok || pw.Worked != 14*h || pw.Breaks != 7*h/4 || pw.ByCategory["htt"] != 14*h {
		t.Errorf("PerWeek over 4 days = %+v, %v", pw, ok)
	}
	if pw, ok := (domain.Stretch{}).PerWeek(); ok || pw.Worked != 0 || pw.ByCategory == nil {
		t.Errorf("PerWeek over no days = %+v, %v; want none", pw, ok)
	}
}

func TestCategoriser_Tagged(t *testing.T) {
	c := figuresCategories(t)
	var names []string
	for _, cat := range c.Tagged() {
		names = append(names, cat.Name)
	}
	if strings.Join(names, " ") != "break htt admin" {
		t.Errorf("Tagged = %v, want the categories with tags, without the built-in working", names)
	}
	var none *domain.Categoriser
	if got := none.Tagged(); len(got) != 0 {
		t.Errorf("no categories: Tagged = %v", got)
	}
}
