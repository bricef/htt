package domain

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
)

// noonOn is date (YYYY-MM-DD) at noon in loc, as periods hold their days.
func noonOn(t *testing.T, loc *time.Location, date string) time.Time {
	t.Helper()
	return localTime(t, loc, date+" 12:00")
}

// sheetOn reads date's log, from lines "HH:MM text" (see dayLog), as a
// timesheet at now.
func sheetOn(t *testing.T, c *Categoriser, loc *time.Location, now time.Time, date string, lines ...string) *Timesheet {
	t.Helper()
	s, err := dayLog(t, loc, date, lines...).Timesheet(c, now)
	if err != nil {
		t.Fatalf("%s: Timesheet: %v", date, err)
	}
	return s
}

// sheetsFor reads the days from first to last (YYYY-MM-DD) as
// timesheets at now, each day's log from logs (none: an empty day).
func sheetsFor(t *testing.T, c *Categoriser, loc *time.Location, now time.Time, first, last string, logs map[string][]string) []*Timesheet {
	t.Helper()
	var days []*Timesheet
	for _, date := range (Period{First: noonOn(t, loc, first), Last: noonOn(t, loc, last)}).Dates() {
		key := date.Format("2006-01-02")
		days = append(days, sheetOn(t, c, loc, now, key, logs[key]...))
	}
	return days
}

// sameTotals reports whether a and b hold the same times, a category
// with no time counting the same as one that isn't there.
func sameTotals(a, b Totals) bool {
	nonzero := func(m map[string]time.Duration) map[string]time.Duration {
		out := map[string]time.Duration{}
		for k, v := range m {
			if v != 0 {
				out[k] = v
			}
		}
		return out
	}
	return a.Worked == b.Worked && a.Breaks == b.Breaks && a.Uncategorised == b.Uncategorised &&
		a.ToMidnight == b.ToMidnight && maps.Equal(nonzero(a.ByCategory), nonzero(b.ByCategory))
}

func showTotals(x Totals) string {
	var cats []string
	for _, k := range slices.Sorted(maps.Keys(x.ByCategory)) {
		if x.ByCategory[k] != 0 {
			cats = append(cats, fmt.Sprintf("%s %v", k, x.ByCategory[k]))
		}
	}
	return fmt.Sprintf("worked %v, breaks %v, uncategorised %v, to midnight %v, [%s]",
		x.Worked, x.Breaks, x.Uncategorised, x.ToMidnight, strings.Join(cats, ", "))
}

func wantTotals(t *testing.T, label string, got, want Totals) {
	t.Helper()
	if !sameTotals(got, want) {
		t.Errorf("%s:\n got %s\nwant %s", label, showTotals(got), showTotals(want))
	}
}

func dates(days []time.Time) string {
	var out []string
	for _, d := range days {
		out = append(out, d.Format("2006-01-02 15:04"))
	}
	return strings.Join(out, " ")
}

func TestWeekOf_MondayToSunday(t *testing.T) {
	london := mustZone(t, "Europe/London")
	cases := []struct {
		at          string
		first, last string
	}{
		{"2026-11-16 08:00", "2026-11-16", "2026-11-22"}, // a Monday
		{"2026-11-19 23:59", "2026-11-16", "2026-11-22"}, // a Thursday
		{"2026-11-22 00:00", "2026-11-16", "2026-11-22"}, // a Sunday ends its week
		{"2026-12-31 12:00", "2026-12-28", "2027-01-03"}, // across a year
		{"2026-03-29 12:00", "2026-03-23", "2026-03-29"}, // the clocks go forward that Sunday
		{"2026-10-25 01:30", "2026-10-19", "2026-10-25"}, // and back, in the hour that repeats
	}
	for _, c := range cases {
		p := WeekOf(localTime(t, london, c.at))
		if !p.First.Equal(noonOn(t, london, c.first)) || !p.Last.Equal(noonOn(t, london, c.last)) {
			t.Errorf("WeekOf(%s) = %s to %s, want %s to %s at noon", c.at,
				p.First.Format("2006-01-02 15:04"), p.Last.Format("2006-01-02 15:04"), c.first, c.last)
		}
		days := p.Dates()
		if len(days) != 7 || days[0].Weekday() != time.Monday || days[6].Weekday() != time.Sunday {
			t.Errorf("WeekOf(%s).Dates() = %s, want Monday to Sunday", c.at, dates(days))
		}
	}
}

func TestMonthOf_TheCalendarMonth(t *testing.T) {
	loc := mustZone(t, "America/New_York")
	cases := []struct {
		at          string
		first, last string
		days        int
	}{
		{"2026-02-10 09:00", "2026-02-01", "2026-02-28", 28},
		{"2028-02-29 09:00", "2028-02-01", "2028-02-29", 29}, // a leap year
		{"2026-04-01 00:00", "2026-04-01", "2026-04-30", 30},
		{"2026-12-31 23:59", "2026-12-01", "2026-12-31", 31},
		{"2026-03-08 03:00", "2026-03-01", "2026-03-31", 31}, // the clocks change in it
	}
	for _, c := range cases {
		p := MonthOf(localTime(t, loc, c.at))
		if !p.First.Equal(noonOn(t, loc, c.first)) || !p.Last.Equal(noonOn(t, loc, c.last)) {
			t.Errorf("MonthOf(%s) = %s to %s, want %s to %s at noon", c.at,
				p.First.Format("2006-01-02 15:04"), p.Last.Format("2006-01-02 15:04"), c.first, c.last)
		}
		if got := len(p.Dates()); got != c.days {
			t.Errorf("MonthOf(%s): %d days, want %d", c.at, got, c.days)
		}
	}
}

func TestPeriod_DatesAreCalendarDays(t *testing.T) {
	// A day the clocks change is 23 or 25 hours long, so the days are
	// counted by their dates, each at noon, not 24 hours apart. In São
	// Paulo the clocks went forward at midnight on 2018-11-04: that
	// midnight never happened.
	for _, c := range []struct{ zone, first, last string }{
		{"Europe/London", "2026-03-23", "2026-03-29"},
		{"Europe/London", "2026-10-19", "2026-11-01"},
		{"America/Sao_Paulo", "2018-10-29", "2018-11-11"},
		{"Australia/Lord_Howe", "2026-04-01", "2026-04-30"}, // clocks back half an hour
	} {
		loc := mustZone(t, c.zone)
		p := Period{First: noonOn(t, loc, c.first), Last: noonOn(t, loc, c.last)}
		days := p.Dates()
		if len(days) == 0 {
			t.Fatalf("%s %s to %s: no days", c.zone, c.first, c.last)
		}
		want := noonOn(t, loc, c.first)
		for i, d := range days {
			if !d.Equal(want) || d.Hour() != 12 {
				t.Fatalf("%s %s to %s: day %d is %s, want %s", c.zone, c.first, c.last, i,
					d.Format(time.RFC3339), want.Format(time.RFC3339))
			}
			want = want.AddDate(0, 0, 1)
		}
		if last := days[len(days)-1]; !last.Equal(p.Last) {
			t.Errorf("%s: last day %s, want %s", c.zone, last, p.Last)
		}
		if !p.Contains(localTime(t, loc, c.first+" 00:30")) || !p.Contains(localTime(t, loc, c.last+" 23:59")) {
			t.Errorf("%s: the period doesn't contain its first and last days", c.zone)
		}
		if p.Contains(want) { // the day after the last
			t.Errorf("%s: the period contains %s, the day after it", c.zone, want)
		}
		if p.Contains(noonOn(t, loc, c.first).AddDate(0, 0, -1)) {
			t.Errorf("%s: the period contains the day before it", c.zone)
		}
	}
}

func TestTimesheet_DateIsTheLogsDay(t *testing.T) {
	// The day's Start is local midnight, which where the clocks go
	// forward at midnight time.Date puts on the day before; Date is the
	// log's own date whatever.
	for _, c := range []struct{ zone, date string }{
		{"Europe/London", "2026-11-19"},
		{"America/Sao_Paulo", "2018-11-04"},
		{"Asia/Kolkata", "2026-01-01"},
	} {
		loc := mustZone(t, c.zone)
		s, err := (&Timelog{Date: noonOn(t, loc, c.date)}).Timesheet(nil, noonOn(t, loc, "2027-01-01"))
		if err != nil {
			t.Fatal(err)
		}
		if !s.Date.Equal(noonOn(t, loc, c.date)) {
			t.Errorf("%s %s: Date = %s, want noon that day", c.zone, c.date, s.Date.Format(time.RFC3339))
		}
	}
}

func TestTimesheet_Totals(t *testing.T) {
	london := mustZone(t, "Europe/London")
	plain := mustCategoriser(t, testCategories())
	onBreak, err := plain.WithDefault("break")
	if err != nil {
		t.Fatal(err)
	}
	now := localTime(t, london, "2026-11-20 10:00")
	h := time.Hour
	m := time.Minute
	cases := []struct {
		name  string
		c     *Categoriser
		date  string
		lines []string
		want  Totals
	}{
		{"an ended day", plain, "2026-11-18", []string{
			"09:00 triage @email", "09:30 fix +htt", "12:00 @lunch", "12:45 fix +htt", "15:00 read the news", "16:00 @end",
		}, Totals{
			Worked: 6*h + 15*m, Breaks: 45 * m, Uncategorised: h,
			ByCategory: map[string]time.Duration{"admin": 30 * m, "htt": 4*h + 45*m, "break": 45 * m},
		}},
		{"a day without @end counts to midnight", plain, "2026-11-18", []string{"20:00 fix +htt"},
			Totals{Worked: 4 * h, ToMidnight: 4 * h, ByCategory: map[string]time.Duration{"htt": 4 * h}}},
		{"only the last entry counts as to midnight", plain, "2026-11-18", []string{"09:00 fix +htt", "10:00 @end", "22:00 @lunch"},
			Totals{Worked: h, Breaks: 2 * h, ToMidnight: 2 * h, ByCategory: map[string]time.Duration{"htt": h, "break": 2 * h}}},
		{"today runs until now, not to midnight", plain, "2026-11-20", []string{"09:00 fix +htt"},
			Totals{Worked: h, ByCategory: map[string]time.Duration{"htt": h}}},
		{"an empty day", plain, "2026-11-19", nil, Totals{}},
		{"uncategorised time in a default that isn't work is a break", onBreak, "2026-11-18", []string{"09:00 read", "10:00 fix +htt", "11:00 @end"},
			Totals{Worked: h, Breaks: h, Uncategorised: h, ByCategory: map[string]time.Duration{"htt": h}}},
		{"the clocks go back: a 25-hour day", plain, "2026-10-25", []string{"00:30 fix +htt"},
			Totals{Worked: 24*h + 30*m, ToMidnight: 24*h + 30*m, ByCategory: map[string]time.Duration{"htt": 24*h + 30*m}}},
		{"the clocks go forward: a 23-hour day", plain, "2026-03-29", []string{"00:30 fix +htt"},
			Totals{Worked: 22*h + 30*m, ToMidnight: 22*h + 30*m, ByCategory: map[string]time.Duration{"htt": 22*h + 30*m}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := sheetOn(t, c.c, london, now, c.date, c.lines...)
			got := s.Totals()
			wantTotals(t, "Totals", got, c.want)
			if got.Logged() != c.want.Worked+c.want.Breaks {
				t.Errorf("Logged = %v, want %v", got.Logged(), c.want.Worked+c.want.Breaks)
			}
		})
	}
}

func TestTotals_PlusAndScale(t *testing.T) {
	a := Totals{Worked: 7 * time.Hour, Breaks: time.Hour, ToMidnight: 2 * time.Hour,
		ByCategory: map[string]time.Duration{"htt": 7 * time.Hour}}
	b := Totals{Worked: time.Hour, Uncategorised: time.Hour, ByCategory: map[string]time.Duration{"htt": 0, "admin": time.Minute}}
	sum := a.Plus(b)
	wantTotals(t, "a+b", sum, Totals{Worked: 8 * time.Hour, Breaks: time.Hour, Uncategorised: time.Hour, ToMidnight: 2 * time.Hour,
		ByCategory: map[string]time.Duration{"htt": 7 * time.Hour, "admin": time.Minute}})
	if a.ByCategory["admin"] != 0 || len(a.ByCategory) != 1 {
		t.Errorf("Plus changed its receiver: %v", a.ByCategory)
	}
	wantTotals(t, "a/7", a.Scale(1, 7), Totals{Worked: time.Hour, Breaks: time.Hour / 7, ToMidnight: 2 * time.Hour / 7,
		ByCategory: map[string]time.Duration{"htt": time.Hour}})
	wantTotals(t, "a per week over 14 days", a.Scale(7, 14), Totals{Worked: 7 * time.Hour / 2, Breaks: time.Hour / 2, ToMidnight: time.Hour,
		ByCategory: map[string]time.Duration{"htt": 7 * time.Hour / 2}})
	wantTotals(t, "a over no days", a.Scale(7, 0), Totals{})
	if a.Scale(7, 0).ByCategory == nil || Sum(nil).ByCategory == nil {
		t.Errorf("a Totals' ByCategory should never be nil")
	}
}

// monthStarts are the months of 2026 that start on each weekday.
var monthStarts = []struct {
	month string
	first time.Weekday
	weeks []int // the days in each week, cut at the month's edges
}{
	{"2026-06", time.Monday, []int{7, 7, 7, 7, 2}},
	{"2026-09", time.Tuesday, []int{6, 7, 7, 7, 3}},
	{"2026-04", time.Wednesday, []int{5, 7, 7, 7, 4}},
	{"2026-10", time.Thursday, []int{4, 7, 7, 7, 6}},
	{"2026-05", time.Friday, []int{3, 7, 7, 7, 7}},
	{"2026-08", time.Saturday, []int{2, 7, 7, 7, 7, 1}},
	{"2026-02", time.Sunday, []int{1, 7, 7, 7, 6}},
	{"2026-03", time.Sunday, []int{1, 7, 7, 7, 7, 2}},
}

func TestWeeks_MondayToSundayCutAtTheEdges(t *testing.T) {
	loc := mustZone(t, "Europe/London")
	now := noonOn(t, loc, "2027-01-01")
	for _, c := range monthStarts {
		t.Run(c.month, func(t *testing.T) {
			p := MonthOf(noonOn(t, loc, c.month+"-01"))
			if p.First.Weekday() != c.first {
				t.Fatalf("%s starts on a %s, not a %s", c.month, p.First.Weekday(), c.first)
			}
			// An hour of htt on each Monday: a week has it once, and the
			// weeks cut short before their Monday have none.
			logs := map[string][]string{}
			for _, d := range p.Dates() {
				if d.Weekday() == time.Monday {
					logs[d.Format("2006-01-02")] = []string{"09:00 fix +htt", "10:00 @end"}
				}
			}
			days := sheetsFor(t, mustCategoriser(t, testCategories()), loc, now,
				p.First.Format("2006-01-02"), p.Last.Format("2006-01-02"), logs)
			weeks := Weeks(days)
			var lengths []int
			for _, w := range weeks {
				lengths = append(lengths, w.Days)
			}
			if !slices.Equal(lengths, c.weeks) {
				t.Fatalf("week lengths %v, want %v", lengths, c.weeks)
			}
			for i, w := range weeks {
				if w.First.Weekday() != time.Monday && i > 0 || w.Last.Weekday() != time.Sunday && i < len(weeks)-1 {
					t.Errorf("week %d runs %s to %s", i, w.First.Format("Mon 2006-01-02"), w.Last.Format("Mon 2006-01-02"))
				}
				if got := len(Period{First: w.First, Last: w.Last}.Dates()); got != w.Days {
					t.Errorf("week %d: %s to %s is %d days, says %d", i, w.First, w.Last, got, w.Days)
				}
				want := Totals{}
				if w.First.Weekday() == time.Monday {
					want = Totals{Worked: time.Hour, ByCategory: map[string]time.Duration{"htt": time.Hour}}
				}
				wantTotals(t, fmt.Sprintf("week %d", i), w.Totals, want)
			}
			if !weeks[0].First.Equal(p.First) || !weeks[len(weeks)-1].Last.Equal(p.Last) {
				t.Errorf("weeks run %s to %s, want the month", weeks[0].First, weeks[len(weeks)-1].Last)
			}
		})
	}
}

func TestHalves_FirstToFifteenthAndTheRest(t *testing.T) {
	loc := mustZone(t, "Asia/Kolkata")
	c := mustCategoriser(t, testCategories())
	now := noonOn(t, loc, "2027-01-01")
	logs := map[string][]string{
		"2026-10-15": {"09:00 fix +htt", "10:00 @end"},
		"2026-10-16": {"09:00 fix +htt", "11:00 @end"},
		"2026-02-15": {"09:00 @lunch", "10:00 @end"},
	}
	cases := []struct {
		first, last   string
		firstDays     int
		secondDays    int
		first1, last1 string
		first2, last2 string
		firstTotals   Totals
		secondTotals  Totals
	}{
		{"2026-10-01", "2026-10-31", 15, 16, "2026-10-01", "2026-10-15", "2026-10-16", "2026-10-31",
			Totals{Worked: time.Hour, ByCategory: map[string]time.Duration{"htt": time.Hour}},
			Totals{Worked: 2 * time.Hour, ByCategory: map[string]time.Duration{"htt": 2 * time.Hour}}},
		{"2026-02-01", "2026-02-28", 15, 13, "2026-02-01", "2026-02-15", "2026-02-16", "2026-02-28",
			Totals{Breaks: time.Hour, ByCategory: map[string]time.Duration{"break": time.Hour}}, Totals{}},
		// The month so far, on the 10th: no second half yet.
		{"2026-10-01", "2026-10-10", 10, 0, "2026-10-01", "2026-10-10", "", "", Totals{}, Totals{}},
	}
	for _, tc := range cases {
		days := sheetsFor(t, c, loc, now, tc.first, tc.last, logs)
		a, b := Halves(days)
		if a.Days != tc.firstDays || b.Days != tc.secondDays {
			t.Errorf("%s to %s: halves of %d and %d days, want %d and %d", tc.first, tc.last, a.Days, b.Days, tc.firstDays, tc.secondDays)
		}
		if a.First.Format("2006-01-02") != tc.first1 || a.Last.Format("2006-01-02") != tc.last1 {
			t.Errorf("%s to %s: first half %s to %s, want %s to %s", tc.first, tc.last, a.First, a.Last, tc.first1, tc.last1)
		}
		if tc.first2 != "" && (b.First.Format("2006-01-02") != tc.first2 || b.Last.Format("2006-01-02") != tc.last2) {
			t.Errorf("%s to %s: second half %s to %s, want %s to %s", tc.first, tc.last, b.First, b.Last, tc.first2, tc.last2)
		}
		if tc.first2 == "" && !b.First.IsZero() {
			t.Errorf("%s to %s: an empty second half starts %s", tc.first, tc.last, b.First)
		}
		wantTotals(t, tc.first+" first half", a.Totals, tc.firstTotals)
		wantTotals(t, tc.first+" second half", b.Totals, tc.secondTotals)
	}
}

func TestRolling_SevenDaysReachingBeforeThePeriod(t *testing.T) {
	loc := mustZone(t, "Europe/London")
	c := mustCategoriser(t, testCategories())
	now := noonOn(t, loc, "2027-01-01")
	h := time.Hour
	days := sheetsFor(t, c, loc, now, "2026-09-25", "2026-10-31", map[string][]string{
		"2026-09-25": {"09:00 fix +htt", "10:00 @end"},
		"2026-10-01": {"09:00 fix +htt", "11:00 @end"},
		"2026-10-02": {"09:00 @email", "09:30 @end"},
		"2026-10-08": {"09:00 fix +htt", "13:00 @end"},
		"2026-10-25": {"20:00 fix +htt"}, // the clocks went back that morning: 4 hours to midnight
	})
	rolling := Rolling(days, 7)
	if len(rolling) != 31 {
		t.Fatalf("%d rolling totals, want one per day of October", len(rolling))
	}
	htt := func(d time.Duration) Totals {
		return Totals{Worked: d, ByCategory: map[string]time.Duration{"htt": d}}
	}
	withAdmin := func(x Totals) Totals {
		x.Worked += 30 * time.Minute
		x.ByCategory["admin"] = 30 * time.Minute
		return x
	}
	unended := func(x Totals) Totals {
		x.ToMidnight = x.Worked
		return x
	}
	// rolling[i] is the 7 days to 1 + i October.
	want := map[int]Totals{
		0:  htt(3 * h),            // 25 Sep to 1 Oct
		1:  withAdmin(htt(2 * h)), // 26 Sep to 2 Oct
		6:  withAdmin(htt(2 * h)), // 1 to 7 Oct
		7:  withAdmin(htt(4 * h)), // 2 to 8 Oct
		8:  htt(4 * h),            // 3 to 9 Oct
		13: htt(4 * h),            // 8 to 14 Oct
		14: {},                    // 9 to 15 Oct: the 8th has left
		23: {},                    // 18 to 24 Oct
		24: unended(htt(4 * h)),   // 19 to 25 Oct
		30: unended(htt(4 * h)),   // 25 to 31 Oct
	}
	for i, w := range want {
		wantTotals(t, fmt.Sprintf("rolling[%d], the 7 days to %s", i, days[i+6].Date.Format("2 Jan")), rolling[i], w)
	}
	if got := Rolling(days[:6], 7); len(got) != 0 {
		t.Errorf("Rolling over 6 days with a window of 7: %d totals, want none", len(got))
	}
}

func TestWeekdayAverages(t *testing.T) {
	loc := mustZone(t, "Europe/London")
	c := mustCategoriser(t, testCategories())
	now := noonOn(t, loc, "2027-01-01")
	h := time.Hour
	// Monday 2 to Thursday 12 November: two of each weekday from Monday
	// to Thursday, one Friday, Saturday and Sunday.
	days := sheetsFor(t, c, loc, now, "2026-11-02", "2026-11-12", map[string][]string{
		"2026-11-02": {"09:00 fix +htt", "11:00 @end"},
		"2026-11-09": {"09:00 fix +htt", "13:00 @end"},
		"2026-11-03": {"12:00 @lunch", "13:00 @end"},
		"2026-11-08": {"09:00 @email", "09:30 @end"},
	})
	avg := WeekdayAverages(days)
	wantDays := []int{2, 2, 2, 2, 1, 1, 1}
	for i, a := range avg {
		if want := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday}[i]; a.Weekday != want {
			t.Errorf("averages[%d] is %s, want %s", i, a.Weekday, want)
		}
		if a.Days != wantDays[i] {
			t.Errorf("%s: %d days, want %d", a.Weekday, a.Days, wantDays[i])
		}
	}
	wantTotals(t, "Monday's total", avg[0].Totals, Totals{Worked: 6 * h, ByCategory: map[string]time.Duration{"htt": 6 * h}})
	wantTotals(t, "Monday's average", avg[0].Average, Totals{Worked: 3 * h, ByCategory: map[string]time.Duration{"htt": 3 * h}})
	wantTotals(t, "Tuesday's average", avg[1].Average, Totals{Breaks: h / 2, ByCategory: map[string]time.Duration{"break": h / 2}})
	wantTotals(t, "Sunday's average", avg[6].Average, Totals{Worked: h / 2, ByCategory: map[string]time.Duration{"admin": h / 2}})
	wantTotals(t, "Friday's average", avg[4].Average, Totals{})

	none := WeekdayAverages(nil)
	if none[0].Days != 0 || !sameTotals(none[0].Average, Totals{}) || none[0].Weekday != time.Monday {
		t.Errorf("averages over no days: %+v", none[0])
	}
}

func TestSummarise(t *testing.T) {
	loc := mustZone(t, "Europe/London")
	c := mustCategoriser(t, testCategories())
	now := noonOn(t, loc, "2027-01-01")
	h := time.Hour
	days := sheetsFor(t, c, loc, now, "2026-10-01", "2026-10-31", map[string][]string{
		"2026-10-01": {"09:00 fix +htt", "17:00 @end"},
		"2026-10-02": {"09:00 fix +htt", "12:00 @lunch", "13:00 read the news", "17:00 @end"},
		"2026-10-05": {"20:00 fix +htt"},
		"2026-10-06": {"20:00 @lunch", "21:00 @end"},
	})
	s := Summarise(days)
	if s.Days != 31 || s.WorkingDays != 3 {
		t.Errorf("%d days, %d working, want 31 and 3", s.Days, s.WorkingDays)
	}
	wantTotals(t, "Totals", s.Totals, Totals{Worked: 19 * h, Breaks: 2 * h, Uncategorised: 4 * h, ToMidnight: 4 * h,
		ByCategory: map[string]time.Duration{"htt": 15 * h, "break": 2 * h}})
	if want := 19 * h * 7 / 31; s.AverageWeek != want {
		t.Errorf("AverageWeek = %v, want %v", s.AverageWeek, want)
	}
	if want := 19 * h / 3; s.AverageWorkingDay != want {
		t.Errorf("AverageWorkingDay = %v, want %v", s.AverageWorkingDay, want)
	}
	if len(s.Unended) != 1 || !s.Unended[0].Equal(noonOn(t, loc, "2026-10-05")) {
		t.Errorf("Unended = %s, want 2026-10-05", dates(s.Unended))
	}

	empty := Summarise(nil)
	if empty.Days != 0 || empty.AverageWeek != 0 || empty.AverageWorkingDay != 0 || len(empty.Unended) != 0 {
		t.Errorf("Summarise(nil) = %+v", empty)
	}
}
