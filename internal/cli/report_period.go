package cli

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bricef/htt/internal/domain"
)

// htt report --week and --month: a week's or a calendar month's tasks
// and time. Every figure is the domain's (domain.Week, domain.Month),
// the ones htt gui shows, so the terminal and the browser agree; this
// file only lays them out.

const reportDateLayout = "2006-01-02"

// reportPeriod prints the activity of the week (--week) or the month
// (--month) that holds args' date, or today's: a heading, the tasks
// sections, and the time section. The figures are read first, so a
// log htt can't read stops it before it prints.
func reportPeriod(args []string) error {
	// The clock to the second, as htt gui's: entries are logged to the
	// second, so every time is whole seconds.
	now := time.Now().Truncate(time.Second)
	date := now
	if len(args) == 1 {
		var err error
		if date, err = periodDate(args[0], now.Location()); err != nil {
			return err
		}
	}
	categories, err := timelogCategoriser()
	if err != nil {
		return err
	}

	if reportWeek {
		w, err := domain.Week(timelogRepo(), categories, date, now)
		if err != nil {
			return err
		}
		name := "the week " + w.Period.First.Format(reportDateLayout) + " to " + w.Period.Last.Format(reportDateLayout)
		if err := reportPeriodTasks(name, w.Period, w.SoFar, now); err != nil {
			return err
		}
		printWeekTime(w, categories)
		return nil
	}
	m, err := domain.Month(timelogRepo(), categories, date, now)
	if err != nil {
		return err
	}
	if err := reportPeriodTasks(m.Period.First.Format("January 2006"), m.Period, m.SoFar, now); err != nil {
		return err
	}
	printMonthTime(m, categories)
	return nil
}

// periodDate reads --week's date or --month's month as a day on the
// calendar (domain.CalendarDate), in loc.
func periodDate(arg string, loc *time.Location) (time.Time, error) {
	if reportWeek {
		d, err := domain.CalendarDate(reportDateLayout, arg, loc)
		if err != nil {
			return time.Time{}, fmt.Errorf("--week: want a date, YYYY-MM-DD, got %q", arg)
		}
		return d, nil
	}
	for _, layout := range []string{"2006-01", reportDateLayout} {
		if d, err := domain.CalendarDate(layout, arg, loc); err == nil {
			return d, nil
		}
	}
	return time.Time{}, fmt.Errorf("--month: want a month, YYYY-MM, or a date in it, YYYY-MM-DD, got %q", arg)
}

// reportPeriodTasks prints the period's heading, saying how far into it
// today is, and its tasks: from its first day to its last, or to now.
func reportPeriodTasks(name string, p domain.Period, soFar int, now time.Time) error {
	switch {
	case p.Contains(now):
		fmt.Printf("Activity in %s (so far: %d of %d days)\n\n", name, soFar, len(p.Dates()))
	case soFar == 0:
		fmt.Printf("Activity in %s (to come)\n\n", name)
	default:
		fmt.Printf("Activity in %s\n\n", name)
	}
	l := p.Last
	until := time.Date(l.Year(), l.Month(), l.Day()+1, 0, 0, 0, 0, l.Location()).Add(-time.Nanosecond)
	if now.Before(until) {
		until = now
	}
	return reportTasks(p.First, until)
}

// printWeekTime is the week's time section: each day so far, as
// --since prints them, and the week's totals; then its figures, as
// htt gui's week summary has them.
func printWeekTime(w *domain.WeekFigures, categories *domain.Categoriser) {
	fmt.Println("Time logged")
	printDays(w.Days[:w.SoFar], w.Summary.Totals, categories)
	s := w.Summary
	fmt.Println()
	printFigures([][2]string{
		{"Worked", reportFigure(s.Totals.Worked)},
		{"Breaks", reportFigure(s.Totals.Breaks)},
		{"Average working day", reportFigureIf(s.AverageWorkingDay, s.WorkingDays > 0)},
		{"Counted to midnight", reportFigure(s.Totals.ToMidnight)},
		{"Days without @end", reportDates(s.Unended)},
	})
}

// printMonthTime is the month's time section: its figures, as htt
// gui's month summary has them; its weeks; the average weekday; and a
// line per category with time.
func printMonthTime(m *domain.MonthFigures, categories *domain.Categoriser) {
	fmt.Println("Time logged")
	s := m.Summary
	def := categories.Default()
	uncategorised := reportFigure(s.Totals.Uncategorised)
	if share, ok := s.UncategorisedShare(def); ok {
		uncategorised = reportPercent(share) + " of worked"
	} else if !def.Work {
		uncategorised += " (counted as " + def.Name + ")"
	}
	printFigures([][2]string{
		{"Worked", reportFigure(s.Totals.Worked)},
		{"Average week", reportFigureIf(s.AverageWeek, s.Days > 0)},
		{"Average working day", reportFigureIf(s.AverageWorkingDay, s.WorkingDays > 0)},
		{"Breaks", reportFigure(s.Totals.Breaks)},
		{"Uncategorised", uncategorised},
		{"Days without @end", reportDates(s.Unended)},
	})

	fmt.Println()
	weeks := [][]string{{"Week", "Days", "Worked", "Breaks"}}
	for _, w := range m.Weeks {
		weeks = append(weeks, []string{
			w.First.Format(reportDateLayout) + " to " + w.Last.Format(reportDateLayout),
			strconv.Itoa(w.Days), reportFigure(w.Totals.Worked), reportFigure(w.Totals.Breaks),
		})
	}
	printTable(weeks)

	fmt.Println()
	weekdays := [][]string{{"Weekday", "Average worked", "Days"}}
	for _, a := range m.Weekdays {
		weekdays = append(weekdays, []string{a.Weekday.String(), reportFigureIf(a.Average.Worked, a.Days > 0), strconv.Itoa(a.Days)})
	}
	printTable(weekdays)

	halves := [2]string{"1-15", fmt.Sprintf("16-%d", m.Period.Last.Day())}
	for i, h := range m.Halves {
		if h.Days > 0 {
			halves[i] = fmt.Sprintf("%d-%d", h.First.Day(), h.Last.Day())
		}
	}
	table := [][]string{{"Category", "Total", "Of worked", "Per week " + halves[0], "Per week " + halves[1], "Change"}}
	for _, r := range m.Table {
		if r.Total == 0 {
			continue
		}
		change := "-"
		if r.HasChange {
			change = reportPercent(r.Change)
			if r.Change > 0 {
				change = "+" + change
			}
		}
		share := "-"
		if r.HasShare {
			share = reportPercent(r.Share)
		}
		table = append(table, []string{r.Name, reportFigure(r.Total), share,
			reportFigureIf(r.PerWeek[0], r.HasPerWeek[0]), reportFigureIf(r.PerWeek[1], r.HasPerWeek[1]), change})
	}
	if len(table) > 1 {
		fmt.Println()
		printTable(table)
	}
}

// reportFigure is a period's figure as the report shows durations
// (formatReportDuration), from d to the second, as htt gui's API gives
// it: an average needn't be whole seconds, and rounded first, the two
// agree to the minute.
func reportFigure(d time.Duration) string {
	return formatReportDuration(d.Round(time.Second))
}

// reportFigureIf is reportFigure(d), or "-" when there's no such figure
// (an average over no days).
func reportFigureIf(d time.Duration, ok bool) string {
	if !ok {
		return "-"
	}
	return reportFigure(d)
}

// reportPercent is x as a whole percentage, rounded.
func reportPercent(x float64) string {
	return fmt.Sprintf("%d%%", int(math.Round(x*100)))
}

// reportDates lists dates, or says there are none.
func reportDates(dates []time.Time) string {
	if len(dates) == 0 {
		return "none"
	}
	out := make([]string, len(dates))
	for i, d := range dates {
		out[i] = d.Format(reportDateLayout)
	}
	return strings.Join(out, ", ")
}

// printFigures prints labelled figures, a line each, the figures lined
// up after the labels.
func printFigures(figures [][2]string) {
	width := 0
	for _, f := range figures {
		width = max(width, utf8.RuneCountInString(f[0]))
	}
	for _, f := range figures {
		fmt.Printf("  %-*s  %s\n", width, f[0], f[1])
	}
}

// printTable prints rows, the first its heading, as columns two spaces
// apart: the first column, names, on the left, and the rest, figures,
// on the right.
func printTable(rows [][]string) {
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], utf8.RuneCountInString(cell))
		}
	}
	for _, row := range rows {
		var b strings.Builder
		fmt.Fprintf(&b, "  %-*s", widths[0], row[0])
		for i, cell := range row[1:] {
			fmt.Fprintf(&b, "  %*s", widths[i+1], cell)
		}
		fmt.Println(b.String())
	}
}
