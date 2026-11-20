package domain

import (
	"fmt"
	"time"
)

// A week's and a month's figures, as `htt gui` draws them and `htt
// report --week` and `--month` print them: the days read from their
// logs, cut at today, and added up with the period functions
// (timesheet_period.go). Both read them from here, so the browser and
// the terminal always agree.

// CalendarDate reads s, a date in layout (a month's is its 1st), as a
// day on the calendar: that day at noon in loc. It isn't read at
// midnight in loc: where the clocks go forward at midnight, that
// midnight doesn't exist, and Go puts it on the day before.
func CalendarDate(layout, s string, loc *time.Location) (time.Time, error) {
	d, err := time.Parse(layout, s) // in UTC, where every midnight happens
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, loc), nil
}

// ReadDays reads each date's log from r as a timesheet at now, its
// entries in c's categories (Timelog.Timesheet). A date after today,
// now's, is a day to come: its log isn't read (one written by hand for
// tomorrow counts for nothing), and it has no time. An error names the
// day. The dates are days at noon in now's zone, as a Period's are.
func ReadDays(r TimelogRepository, c *Categoriser, dates []time.Time, now time.Time) ([]*Timesheet, error) {
	today := noon(now)
	out := make([]*Timesheet, len(dates))
	for i, date := range dates {
		l := &Timelog{Date: date}
		if !date.After(today) {
			var err error
			if l, err = r.Day(date); err != nil {
				return nil, fmt.Errorf("load the timelog for %s: %w", date.Format("2006-01-02"), err)
			}
		}
		sheet, err := l.Timesheet(c, now)
		if err != nil {
			return nil, fmt.Errorf("read the timelog for %s: %w", date.Format("2006-01-02"), err)
		}
		out[i] = sheet
	}
	return out, nil
}

// SoFar is the days, consecutive and in order, up to today, now's: all
// of a period that's over, those to today of the period that holds it,
// today included, and none of one to come. A period's figures are over
// its days so far: averaging over days to come would understate them.
func SoFar(days []*Timesheet, now time.Time) []*Timesheet {
	today := noon(now)
	n := 0
	for n < len(days) && !days[n].Date.After(today) {
		n++
	}
	return days[:n]
}

// WeekFigures is a week's time: its days, and its summary over the days
// so far.
type WeekFigures struct {
	Period  Period
	Days    []*Timesheet // Monday to Sunday; a day to come has no time
	SoFar   int          // how many of Days are today or before: 7 once the week is over, 0 before it starts
	Summary Summary      // over the days so far
}

// Week reads the week, Monday to Sunday, that holds date (a day at noon
// in now's zone) from r at now, and adds it up.
func Week(r TimelogRepository, c *Categoriser, date, now time.Time) (*WeekFigures, error) {
	p := WeekOf(date)
	days, err := ReadDays(r, c, p.Dates(), now)
	if err != nil {
		return nil, err
	}
	soFar := SoFar(days, now)
	return &WeekFigures{Period: p, Days: days, SoFar: len(soFar), Summary: Summarise(soFar)}, nil
}

// MonthFigures is a calendar month's time: its days, and its figures
// over the days so far, but for its weeks.
type MonthFigures struct {
	Period   Period
	Days     []*Timesheet      // the 1st to the last; a day to come has no time
	SoFar    int               // how many of Days are today or before
	Summary  Summary           // over the days so far
	Weeks    []Stretch         // Monday to Sunday, cut at the month's edges: all of them, those to come without time
	Weekdays [7]WeekdayAverage // over the days so far, Monday first
	Rolling  []Totals          // for each day so far, the 7 days to it, the first days' reaching into the month before
	Halves   [2]Stretch        // the 1st to the 15th and the 16th to the end, so far
	Table    []CategoryRow     // a line per category; see CategoryTable
}

// Month reads the calendar month that holds date (a day at noon in
// now's zone) from r at now, with the six days before it for its first
// days' rolling weeks, and adds it up.
func Month(r TimelogRepository, c *Categoriser, date, now time.Time) (*MonthFigures, error) {
	p := MonthOf(date)
	all, err := ReadDays(r, c, Period{First: p.First.AddDate(0, 0, -6), Last: p.Last}.Dates(), now)
	if err != nil {
		return nil, err
	}
	days := all[6:]
	soFar := SoFar(days, now)
	m := &MonthFigures{
		Period:   p,
		Days:     days,
		SoFar:    len(soFar),
		Summary:  Summarise(soFar),
		Weeks:    Weeks(days),
		Weekdays: WeekdayAverages(soFar),
		Rolling:  Rolling(all[:6+len(soFar)], 7),
	}
	m.Halves[0], m.Halves[1] = Halves(soFar)
	m.Table = CategoryTable(c, m.Summary.Totals, m.Halves[0], m.Halves[1])
	return m, nil
}

// PerWeek is the stretch's time per week, its Totals × 7 ÷ its Days;
// false for a stretch of no days, which has none.
func (s Stretch) PerWeek() (Totals, bool) {
	if s.Days == 0 {
		return Totals{ByCategory: map[string]time.Duration{}}, false
	}
	return s.Totals.Scale(7, s.Days), true
}

// UncategorisedShare is the uncategorised time's share of the worked
// time; false when it isn't worked time (def, the default category,
// isn't work) or no time was worked.
func (s Summary) UncategorisedShare(def Category) (float64, bool) {
	if !def.Work {
		return 0, false
	}
	return share(s.Totals.Uncategorised, s.Totals.Worked)
}

// share is part ÷ whole; false when whole has no time.
func share(part, whole time.Duration) (float64, bool) {
	if whole <= 0 {
		return 0, false
	}
	return float64(part) / float64(whole), true
}

// Tagged lists the categories an entry's tags can put it in, in c's
// order. A category without tags (the built-in default, or a default
// the config gave none) has no time of its own, only uncategorised
// time, so it isn't one of them.
func (c *Categoriser) Tagged() []Category {
	var out []Category
	for _, cat := range c.Categories() {
		if len(cat.Tags) > 0 {
			out = append(out, cat)
		}
	}
	return out
}

// CategoryRow is a category's line in a month's table: its time, its
// share of the worked time, its time per week in each half, and the
// change from the first half to the second.
type CategoryRow struct {
	Name          string // the category's; UncategorisedName for the time no category matched
	Work          bool   // its time is worked time; uncategorised time's is when the default category's is
	Uncategorised bool
	Total         time.Duration
	Share         float64          // of the worked time, if HasShare: a work category, and some time worked
	HasShare      bool             //
	PerWeek       [2]time.Duration // in each half: its time × 7 ÷ the half's days, if HasPerWeek
	HasPerWeek    [2]bool          // the half has days; the second has none early in a month
	Change        float64          // the second half's time per week ÷ the first's − 1, if HasChange
	HasChange     bool             // both halves have days, and the first some time in the category
}

// CategoryTable is a line per category, those with tags in c's order
// (Categoriser.Tagged), then uncategorised: its time in total, the
// period's, and in first and second, its halves.
func CategoryTable(c *Categoriser, total Totals, first, second Stretch) []CategoryRow {
	type column struct {
		name          string
		work          bool
		uncategorised bool
	}
	var columns []column
	for _, cat := range c.Tagged() {
		columns = append(columns, column{cat.Name, cat.Work, false})
	}
	columns = append(columns, column{UncategorisedName, c.Default().Work, true})

	timeIn := func(t Totals, col column) time.Duration {
		if col.uncategorised {
			return t.Uncategorised
		}
		return t.ByCategory[col.name]
	}
	var halves [2]Totals
	var have [2]bool
	for i, h := range []Stretch{first, second} {
		halves[i], have[i] = h.PerWeek()
	}
	rows := make([]CategoryRow, 0, len(columns))
	for _, col := range columns {
		row := CategoryRow{Name: col.name, Work: col.work, Uncategorised: col.uncategorised, Total: timeIn(total, col)}
		if col.work {
			row.Share, row.HasShare = share(row.Total, total.Worked)
		}
		for i := range halves {
			if have[i] {
				row.PerWeek[i], row.HasPerWeek[i] = timeIn(halves[i], col), true
			}
		}
		if have[0] && have[1] && row.PerWeek[0] > 0 {
			row.Change, row.HasChange = float64(row.PerWeek[1])/float64(row.PerWeek[0])-1, true
		}
		rows = append(rows, row)
	}
	return rows
}
