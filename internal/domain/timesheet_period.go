package domain

import (
	"maps"
	"time"
)

// Period aggregation: days' timesheets added up over a week or a month,
// split into weeks and halves, averaged by weekday, and totalled over a
// rolling window. Pure functions over timesheets, one per calendar day
// in order; `htt gui` draws its views from them.

// Period is a run of calendar days, First to Last, each held as its date
// at noon in the local zone, as Timesheet.Date holds a day: noon is on
// its date whatever the clocks do at midnight, and the days are counted
// by their dates, not 24 hours apart (a day the clocks change is 23 or
// 25 hours long).
type Period struct {
	First, Last time.Time
}

// noon is t's date at noon in t's location.
func noon(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, t.Location())
}

// weekdayIndex counts the days of the week from Monday, 0, to Sunday, 6.
func weekdayIndex(date time.Time) int {
	return (int(date.Weekday()) + 6) % 7
}

// WeekOf is the week that holds date, Monday to Sunday, by date's day
// in its location.
func WeekOf(date time.Time) Period {
	first := noon(date).AddDate(0, 0, -weekdayIndex(date))
	return Period{First: first, Last: first.AddDate(0, 0, 6)}
}

// MonthOf is the calendar month that holds date, by date's day in its
// location.
func MonthOf(date time.Time) Period {
	first := time.Date(date.Year(), date.Month(), 1, 12, 0, 0, 0, date.Location())
	return Period{First: first, Last: first.AddDate(0, 1, -1)}
}

// Dates lists the period's days, First to Last, each at noon.
func (p Period) Dates() []time.Time {
	var days []time.Time
	for d := noon(p.First); !d.After(p.Last); d = d.AddDate(0, 0, 1) {
		days = append(days, d)
	}
	return days
}

// Contains reports whether date's day, in the period's zone, is one of
// the period's days.
func (p Period) Contains(date time.Time) bool {
	d := noon(date.In(p.First.Location()))
	return !d.Before(p.First) && !d.After(p.Last)
}

// Totals is time from timesheets added up: a day's, or a period's.
// Worked and Breaks split the time logged by whether its category
// counts as work; ByCategory and Uncategorised split it by category, as
// Timesheet.ByCategory and Timesheet.Uncategorised do. ToMidnight is
// the time of each day without @end's last entry, counted to 24:00, so
// part of the rest: the time in doubt.
//
// ByCategory is never nil in a Totals this file returns.
type Totals struct {
	Worked        time.Duration
	Breaks        time.Duration
	ByCategory    map[string]time.Duration
	Uncategorised time.Duration
	ToMidnight    time.Duration
}

// Totals is the day's time added up: worked, breaks, by category,
// uncategorised, and, on a day without @end, its last entry's time to
// 24:00.
func (s *Timesheet) Totals() Totals {
	t := Totals{
		Worked:        s.Worked(),
		ByCategory:    s.ByCategory(),
		Uncategorised: s.Uncategorised(),
	}
	var logged time.Duration
	for _, iv := range s.Intervals {
		logged += iv.Duration()
	}
	t.Breaks = logged - t.Worked
	if s.Status == DayUnended && len(s.Intervals) > 0 {
		t.ToMidnight = s.Intervals[len(s.Intervals)-1].Duration()
	}
	return t
}

// Logged is all the time: worked and breaks.
func (t Totals) Logged() time.Duration {
	return t.Worked + t.Breaks
}

// Plus is t and u added up. Neither changes.
func (t Totals) Plus(u Totals) Totals {
	sum := Totals{
		Worked:        t.Worked + u.Worked,
		Breaks:        t.Breaks + u.Breaks,
		ByCategory:    make(map[string]time.Duration, len(t.ByCategory)),
		Uncategorised: t.Uncategorised + u.Uncategorised,
		ToMidnight:    t.ToMidnight + u.ToMidnight,
	}
	maps.Copy(sum.ByCategory, t.ByCategory)
	for name, d := range u.ByCategory {
		sum.ByCategory[name] += d
	}
	return sum
}

// Scale is t times num over den, each time to the nanosecond, rounded
// down: Scale(1, n) is the average of n days, Scale(7, n) the time per
// week over n days. Over no days (den 0) there is no time.
func (t Totals) Scale(num, den int) Totals {
	scaled := Totals{ByCategory: map[string]time.Duration{}}
	if den == 0 {
		return scaled
	}
	by := func(d time.Duration) time.Duration {
		return d * time.Duration(num) / time.Duration(den)
	}
	scaled.Worked = by(t.Worked)
	scaled.Breaks = by(t.Breaks)
	scaled.Uncategorised = by(t.Uncategorised)
	scaled.ToMidnight = by(t.ToMidnight)
	for name, d := range t.ByCategory {
		scaled.ByCategory[name] = by(d)
	}
	return scaled
}

// Sum is the days' time added up.
func Sum(days []*Timesheet) Totals {
	sum := Totals{ByCategory: map[string]time.Duration{}}
	for _, s := range days {
		sum = sum.Plus(s.Totals())
	}
	return sum
}

// Stretch is consecutive days of a period, First to Last, and their
// time: a week of a month, or half of it. A stretch of no days has zero
// dates.
type Stretch struct {
	First, Last time.Time
	Days        int
	Totals      Totals
}

// StretchOf is the days, consecutive and in order, as a stretch.
func StretchOf(days []*Timesheet) Stretch {
	s := Stretch{Days: len(days), Totals: Sum(days)}
	if len(days) > 0 {
		s.First, s.Last = days[0].Date, days[len(days)-1].Date
	}
	return s
}

// Weeks splits consecutive days into weeks, Monday to Sunday: the first
// and the last are cut at the days' edges, as a month's are.
func Weeks(days []*Timesheet) []Stretch {
	var weeks []Stretch
	from := 0
	for i := 1; i <= len(days); i++ {
		if i == len(days) || !WeekOf(days[i].Date).First.Equal(WeekOf(days[from].Date).First) {
			weeks = append(weeks, StretchOf(days[from:i]))
			from = i
		}
	}
	return weeks
}

// Halves splits a month's days, consecutive and in order, into the 1st
// to the 15th and the 16th to the end. Of a month so far, the second
// half can have no days yet.
func Halves(days []*Timesheet) (first, second Stretch) {
	cut := 0
	for cut < len(days) && days[cut].Date.Day() <= 15 {
		cut++
	}
	return StretchOf(days[:cut]), StretchOf(days[cut:])
}

// Rolling is the rolling total over window days: for each day from
// days[window-1] on, the time of the window days up to and including
// it. Pass the window-1 days before a period to have a total for each
// of its days. Fewer days than the window give none.
func Rolling(days []*Timesheet, window int) []Totals {
	var out []Totals
	for end := window; end <= len(days); end++ {
		out = append(out, Sum(days[end-window:end]))
	}
	return out
}

// WeekdayAverage is a weekday's time over a period: its days, their
// total, and the average day.
type WeekdayAverage struct {
	Weekday time.Weekday
	Days    int
	Totals  Totals
	Average Totals // Totals ÷ Days; no time when there are no days
}

// WeekdayAverages is each weekday's time over the days, Monday first.
func WeekdayAverages(days []*Timesheet) [7]WeekdayAverage {
	var byDay [7][]*Timesheet
	for _, s := range days {
		i := weekdayIndex(s.Date)
		byDay[i] = append(byDay[i], s)
	}
	var out [7]WeekdayAverage
	for i, mine := range byDay {
		total := Sum(mine)
		out[i] = WeekdayAverage{
			Weekday: time.Weekday((i + 1) % 7),
			Days:    len(mine),
			Totals:  total,
			Average: total.Scale(1, len(mine)),
		}
	}
	return out
}

// Summary is a period's time in a few figures.
type Summary struct {
	Days              int           // the days summarised
	WorkingDays       int           // those with worked time
	Totals            Totals        // their time
	AverageWeek       time.Duration // worked time per 7 days: Worked × 7 ÷ Days
	AverageWorkingDay time.Duration // worked time per working day: Worked ÷ WorkingDays
	Unended           []time.Time   // the dates of the days without @end
}

// Summarise sums up the days, consecutive and in order. Averages over
// no days are zero.
func Summarise(days []*Timesheet) Summary {
	s := Summary{Days: len(days), Totals: Sum(days)}
	for _, d := range days {
		if d.Worked() > 0 {
			s.WorkingDays++
		}
		if d.Status == DayUnended {
			s.Unended = append(s.Unended, d.Date)
		}
	}
	if s.Days > 0 {
		s.AverageWeek = s.Totals.Worked * 7 / time.Duration(s.Days)
	}
	if s.WorkingDays > 0 {
		s.AverageWorkingDay = s.Totals.Worked / time.Duration(s.WorkingDays)
	}
	return s
}
