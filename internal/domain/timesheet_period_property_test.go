package domain

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

// genMonth is a generated month: the timesheets from six days before
// its first day (so the first days have a full rolling week) to its
// last day so far, and what the test knows about it independently.
type genMonth struct {
	month  Period
	before []*Timesheet // the six days before the month
	days   []*Timesheet // the month's days, up to today when it's this month
	now    time.Time
}

// generateMonth writes a random month's logs in a random zone, one
// with clock changes now and then: any month of 2018 to 2027, past or,
// one time in four, this month, the clock at a random minute of one of
// its days. Each day has up to eight entries at whole minutes from a
// small set of texts with the test categories' tags, near misses and
// none; an @end now and then among them, and at the end of the day half
// the time. A day after today has no log, as the GUI loads none.
func generateMonth(t *testing.T, r *rand.Rand, zones []*time.Location, c *Categoriser) genMonth {
	t.Helper()
	loc := zones[r.IntN(len(zones))]
	first := time.Date(2018+r.IntN(10), time.Month(1+r.IntN(12)), 1, 12, 0, 0, 0, loc)
	g := genMonth{month: MonthOf(first)}
	dates := g.month.Dates()
	if r.IntN(4) == 0 { // this month
		today := dates[r.IntN(len(dates))]
		midnight := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
		minutes := int(midnight.AddDate(0, 0, 1).Sub(midnight) / time.Minute)
		g.now = midnight.Add(time.Duration(r.IntN(minutes)) * time.Minute)
	} else {
		g.now = g.month.Last.AddDate(0, 0, 1+r.IntN(40))
	}

	texts := []string{"fix +htt", "@lunch", "@break coffee", "triage @email", "invoices +admin", "read the news",
		"@Break", "+HTT", "fix the @end bug", "@lunch +htt"}
	sheet := func(date time.Time) *Timesheet {
		l := &Timelog{Date: date}
		start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
		end := start.AddDate(0, 0, 1)
		if end.After(g.now) {
			end = g.now // today: nothing logged after now
		}
		var stamps []time.Time
		if span := int(end.Sub(start) / time.Minute); span > 0 {
			for range r.IntN(9) {
				stamps = append(stamps, start.Add(time.Duration(r.IntN(span))*time.Minute))
			}
		}
		slices.SortFunc(stamps, func(a, b time.Time) int { return a.Compare(b) })
		for i, at := range stamps {
			text := texts[r.IntN(len(texts))]
			if r.IntN(8) == 0 || i == len(stamps)-1 && r.IntN(2) == 0 {
				text = DayEndEntry
			}
			// As Append writes it, parsed once (Annotate would parse it again).
			l.Entries = append(l.Entries, mustTask(t, text+" "+TimelogTimestampLabel+":"+at.UTC().Format(time.RFC3339)))
		}
		s, err := l.Timesheet(c, g.now)
		if err != nil {
			t.Fatalf("Timesheet: %v", err)
		}
		return s
	}
	for _, d := range (Period{First: g.month.First.AddDate(0, 0, -6), Last: g.month.First.AddDate(0, 0, -1)}).Dates() {
		g.before = append(g.before, sheet(d))
	}
	today := time.Date(g.now.Year(), g.now.Month(), g.now.Day(), 12, 0, 0, 0, loc)
	for _, d := range dates {
		if d.After(today) {
			break
		}
		g.days = append(g.days, sheet(d))
	}
	return g
}

// oracleTotals adds up the days' intervals one by one, without
// Timesheet.Totals: work or not by the interval's category, in its
// category or uncategorised, and the last interval of a day without
// @end as time to midnight.
func oracleTotals(days []*Timesheet) Totals {
	o := Totals{ByCategory: map[string]time.Duration{}}
	for _, s := range days {
		for i, iv := range s.Intervals {
			d := iv.End.Sub(iv.Start)
			if iv.Category.Work {
				o.Worked += d
			} else {
				o.Breaks += d
			}
			if iv.Uncategorised {
				o.Uncategorised += d
			} else {
				o.ByCategory[iv.Category.Name] += d
			}
			if s.Status == DayUnended && i == len(s.Intervals)-1 {
				o.ToMidnight += d
			}
		}
	}
	return o
}

// within reports whether every field of avg × n is within n
// nanoseconds of total's: an average is a total divided by a count,
// to the nanosecond.
func within(avg Totals, n int, total Totals) bool {
	close := func(a, b time.Duration) bool {
		diff := a*time.Duration(n) - b
		return diff <= 0 && diff > -time.Duration(n)
	}
	if !close(avg.Worked, total.Worked) || !close(avg.Breaks, total.Breaks) ||
		!close(avg.Uncategorised, total.Uncategorised) || !close(avg.ToMidnight, total.ToMidnight) {
		return false
	}
	for k, v := range total.ByCategory {
		if !close(avg.ByCategory[k], v) {
			return false
		}
	}
	return true
}

// TestPeriods_Properties checks the period aggregates over generated
// months against direct sums: the days add up to the month; the weeks
// are Monday to Sunday, cut at the month's edges, and partition it; each
// rolling total is the direct sum of its 7 days, reaching into the
// month before; each weekday's average times its count of days is its
// total, and the counts and totals add up to the month's; the halves
// split it at the 15th; and the summary's figures follow from its
// totals. Months start on every weekday, in zones with and without
// clock changes.
func TestPeriods_Properties(t *testing.T) {
	var zones []*time.Location
	for _, name := range []string{"UTC", "Europe/London", "America/New_York", "Asia/Kolkata", "Australia/Lord_Howe", "America/Sao_Paulo"} {
		zones = append(zones, mustZone(t, name))
	}
	plain := mustCategoriser(t, testCategories())
	onBreak, err := plain.WithDefault("break")
	if err != nil {
		t.Fatal(err)
	}
	r := rand.New(rand.NewPCG(20261119, 9))
	starts := map[time.Weekday]bool{}
	for i := range 150 {
		c := []*Categoriser{plain, onBreak, nil}[i%3]
		g := generateMonth(t, r, zones, c)
		starts[g.month.First.Weekday()] = true
		fail := func(format string, args ...any) {
			t.Helper()
			t.Fatalf("case %d (%s, %s, now %s): %s", i, g.month.First.Format("2006-01"), g.month.First.Location(),
				g.now.Format("2006-01-02 15:04"), fmt.Sprintf(format, args...))
		}
		days := g.days
		total := oracleTotals(days)

		// The days sum to the month.
		if got := Sum(days); !sameTotals(got, total) {
			fail("Sum = %s, want %s", showTotals(got), showTotals(total))
		}
		var each Totals
		for _, d := range days {
			each = each.Plus(d.Totals())
			if !sameTotals(d.Totals(), oracleTotals([]*Timesheet{d})) {
				fail("%s: Totals = %s, want %s", d.Date.Format("2006-01-02"), showTotals(d.Totals()), showTotals(oracleTotals([]*Timesheet{d})))
			}
		}
		if !sameTotals(each, total) {
			fail("the days' totals add to %s, want %s", showTotals(each), showTotals(total))
		}
		if len(days) == 0 {
			continue
		}

		// The weeks partition the month: in order, without gaps, each
		// within one Monday-to-Sunday week, starting on a Monday except
		// the first and ending on a Sunday except the last.
		weeks := Weeks(days)
		var weekSum Totals
		next := days[0].Date
		n := 0
		for j, w := range weeks {
			if !w.First.Equal(next) {
				fail("week %d starts %s, want %s", j, w.First, next)
			}
			if !WeekOf(w.First).First.Equal(WeekOf(w.Last).First) {
				fail("week %d, %s to %s, spans two weeks", j, w.First, w.Last)
			}
			if j > 0 && w.First.Weekday() != time.Monday || j < len(weeks)-1 && w.Last.Weekday() != time.Sunday {
				fail("week %d runs %s to %s", j, w.First.Format("Mon 2006-01-02"), w.Last.Format("Mon 2006-01-02"))
			}
			if want := oracleTotals(days[n : n+w.Days]); !sameTotals(w.Totals, want) {
				fail("week %d: %s, want %s", j, showTotals(w.Totals), showTotals(want))
			}
			if !days[n+w.Days-1].Date.Equal(w.Last) {
				fail("week %d ends %s, its last day is %s", j, w.Last, days[n+w.Days-1].Date)
			}
			weekSum = weekSum.Plus(w.Totals)
			n += w.Days
			next = w.Last.AddDate(0, 0, 1)
		}
		if n != len(days) || !sameTotals(weekSum, total) {
			fail("the weeks hold %d days and %s, want %d and %s", n, showTotals(weekSum), len(days), showTotals(total))
		}

		// Each rolling total is the sum of its 7 days, the first ones
		// reaching into the month before.
		span := append(slices.Clone(g.before), days...)
		rolling := Rolling(span, 7)
		if len(rolling) != len(days) {
			fail("%d rolling totals for %d days", len(rolling), len(days))
		}
		for j, got := range rolling {
			window := span[j : j+7]
			if !window[6].Date.Equal(days[j].Date) || !window[0].Date.Equal(days[j].Date.AddDate(0, 0, -6)) {
				fail("rolling[%d]'s window is %s to %s", j, window[0].Date, window[6].Date)
			}
			if want := oracleTotals(window); !sameTotals(got, want) {
				fail("rolling[%d], to %s: %s, want %s", j, days[j].Date.Format("2006-01-02"), showTotals(got), showTotals(want))
			}
		}

		// Weekday averages times their counts are their totals, which
		// add up to the month's.
		var counted int
		var weekdaySum Totals
		for j, a := range WeekdayAverages(days) {
			var mine []*Timesheet
			for _, d := range days {
				if (int(d.Date.Weekday())+6)%7 == j {
					mine = append(mine, d)
				}
			}
			if a.Days != len(mine) || !sameTotals(a.Totals, oracleTotals(mine)) {
				fail("%s: %d days, %s; want %d, %s", a.Weekday, a.Days, showTotals(a.Totals), len(mine), showTotals(oracleTotals(mine)))
			}
			if a.Days > 0 && !within(a.Average, a.Days, a.Totals) {
				fail("%s: average %s × %d isn't %s", a.Weekday, showTotals(a.Average), a.Days, showTotals(a.Totals))
			}
			counted += a.Days
			weekdaySum = weekdaySum.Plus(a.Totals)
		}
		if counted != len(days) || !sameTotals(weekdaySum, total) {
			fail("the weekdays hold %d days and %s, want %d and %s", counted, showTotals(weekdaySum), len(days), showTotals(total))
		}

		// The halves split the month at the 15th.
		first, second := Halves(days)
		cut := 0
		for cut < len(days) && days[cut].Date.Day() <= 15 {
			cut++
		}
		if first.Days != cut || second.Days != len(days)-cut ||
			!sameTotals(first.Totals, oracleTotals(days[:cut])) || !sameTotals(second.Totals, oracleTotals(days[cut:])) {
			fail("halves of %d and %d days (%s; %s), want %d and %d", first.Days, second.Days,
				showTotals(first.Totals), showTotals(second.Totals), cut, len(days)-cut)
		}

		// The summary follows from the month's totals.
		s := Summarise(days)
		working := 0
		var unended []time.Time
		for _, d := range days {
			if oracleTotals([]*Timesheet{d}).Worked > 0 {
				working++
			}
			if d.Status == DayUnended {
				unended = append(unended, d.Date)
			}
		}
		if s.Days != len(days) || s.WorkingDays != working || !sameTotals(s.Totals, total) || !slices.EqualFunc(s.Unended, unended, time.Time.Equal) {
			fail("summary: %d days, %d working, %s, unended %s; want %d, %d, %s, %s", s.Days, s.WorkingDays,
				showTotals(s.Totals), dates(s.Unended), len(days), working, showTotals(total), dates(unended))
		}
		if !within(Totals{Worked: s.AverageWeek}, s.Days, Totals{Worked: total.Worked * 7}) {
			fail("average week %v × %d days isn't 7 × %v", s.AverageWeek, s.Days, total.Worked)
		}
		if working > 0 && !within(Totals{Worked: s.AverageWorkingDay}, working, Totals{Worked: total.Worked}) {
			fail("average working day %v × %d isn't %v", s.AverageWorkingDay, working, total.Worked)
		}
	}
	if len(starts) != 7 {
		t.Errorf("the generated months started on only %d weekdays", len(starts))
	}
}
