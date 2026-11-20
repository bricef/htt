package e2e

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// htt report --week [DATE] and --month [MONTH]: a week or a calendar
// month, with the figures htt gui shows (the domain's Week and Month).
// The logs below are in Asia/Kolkata (timelogEnv), in planConfig's
// categories.

// periodLogs are the week of Monday 2 November 2026 and October 2026,
// with a day either side of each that they leave out.
var periodLogs = map[string][]string{
	"2026-09-28": {"09:00 fix +htt", "17:00 @end"}, // before October
	// October: Thursday the 1st to Saturday the 31st.
	"2026-10-01": {"09:00 triage @email", "09:30 parser +htt", "12:00 @lunch", "12:45 parser +htt", "17:00 @end"},
	"2026-10-02": {"10:00 standup", "10:15 release +htt", "16:00 @end"},
	"2026-10-05": {"09:00 fix +htt", "12:00 @lunch", "13:00 docs", "17:00 @end"},
	"2026-10-15": {"09:00 parser +htt"}, // no @end
	"2026-10-16": {"09:00 reading the news", "10:00 @end"},
	"2026-10-20": {"09:00 mail @email", "11:00 review +htt", "12:30 @lunch", "13:30 review +htt", "17:30 @end"},
	"2026-10-27": {"09:00 docs +htt", "11:30 @end"},
	"2026-10-31": {"10:00 tidy +admin", "11:00 @end"},
	"2026-11-01": {"09:00 fix +htt", "10:00 @end"}, // after October, before the week
	// The week: Monday the 2nd to Sunday the 8th.
	"2026-11-02": {"09:00 review the PR +htt", "12:00 @lunch", "13:00 mail @email", "17:00 @end"},
	"2026-11-03": {"09:00 writing the report", "12:30 @lunch", "13:15 fix +htt", "18:00 @end"},
	"2026-11-04": {"09:00 review +htt", "13:00 mail @email"}, // no @end
	"2026-11-06": {"10:00 @end"},                             // an @end alone: no time
	"2026-11-07": {"10:00 tidy +admin", "11:30 @end"},
	"2026-11-09": {"09:00 fix +htt", "10:00 @end"}, // the week after
}

// writePeriodData writes periodLogs and tasks added, completed and
// deleted in and around the week and the month.
func (e *env) writePeriodData(loc *time.Location) {
	e.t.Helper()
	for date, lines := range periodLogs {
		e.writeTimelog(loc, date, lines...)
	}
	e.writeData("todo.txt", "2026-11-03 write the plan +htt\n2026-10-20 old task\n2026-11-10 later task\n")
	e.writeData("done.txt", "x 2026-11-04 2026-10-30 ship v0.5 context:todo\n"+
		"x 2026-11-09 2026-11-02 fix the bug context:todo\n"+
		"x 2026-10-12 2026-09-30 tidy the backlog context:todo\n")
	e.writeData("archive.txt", "2026-11-01 drop me archived-from:todo deleted-on:2026-11-05\n"+
		"2026-09-01 old archived-from:todo deleted-on:2026-10-01\n")
}

func TestReportWeek_APastWeek(t *testing.T) {
	// 31h 45m worked over 4 working days: 7h 56m 15s a day.
	e, loc := timelogEnv(t)
	e.writeConfig(planConfig)
	e.writePeriodData(loc)
	r := e.mustRun("report", "--week", "2026-11-04")
	assertEqual(t, "report --week 2026-11-04", r.stdout, `Activity in the week 2026-11-02 to 2026-11-08

Completed (1)
  todo:
    ship v0.5                                          2026-11-04

Added (2)
  todo:
    fix the bug                                        2026-11-02
    write the plan +htt                                2026-11-03

Deleted (1)
  todo:
    drop me                                            2026-11-05

Time logged
  2026-11-02   7h 0m worked
    break            1h 0m
    htt              3h 0m
    admin            4h 0m
  2026-11-03  8h 15m worked
    break              45m
    htt             4h 45m
    uncategorised   3h 30m  (counted as working)
  2026-11-04  15h 0m worked (no @end: counted to 24:00)
              end the day with htt log end --at "2026-11-04 HH:MM"
    htt              4h 0m
    admin           11h 0m
  2026-11-07  1h 30m worked
    admin           1h 30m
  Total: 31h 45m worked
    break           1h 45m
    htt            11h 45m
    admin          16h 30m
    uncategorised   3h 30m  (counted as working)

  Worked               31h 45m
  Breaks               1h 45m
  Average working day  7h 56m
  Counted to midnight  11h 0m
  Days without @end    2026-11-04
`)
	assertEqual(t, "stderr", r.stderr, "")

	// Any day of the week is that week; the date is a calendar date.
	for _, date := range []string{"2026-11-02", "2026-11-08"} {
		assertEqual(t, "report --week "+date, e.mustRun("report", "--week", date).stdout, r.stdout)
	}
}

func TestReportMonth_APastMonth(t *testing.T) {
	// 47h 15m worked over 31 days, 8 of them working days.
	e, loc := timelogEnv(t)
	e.writeConfig(planConfig)
	e.writePeriodData(loc)
	r := e.mustRun("report", "--month", "2026-10")
	assertEqual(t, "report --month 2026-10", r.stdout, `Activity in October 2026

Completed (1)
  todo:
    tidy the backlog                                   2026-10-12

Added (2)
  todo:
    old task                                           2026-10-20
    ship v0.5                                          2026-10-30

Deleted (1)
  todo:
    old                                                2026-10-01

Time logged
  Worked               47h 15m
  Average week         10h 40m
  Average working day  5h 54m
  Breaks               2h 45m
  Uncategorised        11% of worked
  Days without @end    2026-10-15

  Week                      Days   Worked  Breaks
  2026-10-01 to 2026-10-04     4  13h 15m     45m
  2026-10-05 to 2026-10-11     7    7h 0m   1h 0m
  2026-10-12 to 2026-10-18     7   16h 0m      0m
  2026-10-19 to 2026-10-25     7   7h 30m   1h 0m
  2026-10-26 to 2026-10-31     6   3h 30m      0m

  Weekday    Average worked  Days
  Monday             1h 45m     4
  Tuesday            2h 30m     4
  Wednesday              0m     4
  Thursday           4h 27m     5
  Friday             1h 24m     5
  Saturday              12m     5
  Sunday                 0m     4

  Category         Total  Of worked  Per week 1-15  Per week 16-31  Change
  break           2h 45m          -            49m             26m    -46%
  htt            38h 30m        81%        14h 14m          3h 30m    -75%
  admin           3h 30m         7%            14m          1h 18m   +463%
  uncategorised   5h 15m        11%         1h 59m             26m    -78%
`)
	assertEqual(t, "stderr", r.stderr, "")

	// A date in the month is that month.
	assertEqual(t, "report --month 2026-10-31", e.mustRun("report", "--month", "2026-10-31").stdout, r.stdout)
}

func TestReportMonth_UncategorisedTimeThatIsntWorked(t *testing.T) {
	// The default category isn't work: uncategorised time isn't worked
	// time, so it has no share of it, and says what it counts as.
	e, loc := timelogEnv(t)
	e.writeConfig(strings.Replace(planConfig, "default_category: working", "default_category: break", 1))
	e.writeTimelog(loc, "2026-10-05", "09:00 fix +htt", "12:00 docs", "13:00 @end")
	out := timeSection(t, e.mustRun("report", "--month", "2026-10").stdout)
	assertInOrder(t, "time section", out,
		"  Worked               3h 0m\n",
		"  Breaks               1h 0m\n",
		"  Uncategorised        1h 0m (counted as break)\n",
		"  Category       Total  Of worked  Per week 1-15  Per week 16-31  Change\n",
		"  htt            3h 0m       100%         1h 24m              0m   -100%\n",
		"  uncategorised  1h 0m          -            28m              0m   -100%\n")
	// break's own tags matched nothing: it has no line of its own.
	if strings.Contains(out, "\n  break ") {
		t.Errorf("a line for break, which its tags gave no time:\n%s", out)
	}
}

func TestReportWeek_NothingLogged(t *testing.T) {
	e, _ := timelogEnv(t)
	assertEqual(t, "report --week 2026-11-04", timeSection(t, e.mustRun("report", "--week", "2026-11-04").stdout), `Time logged
  (no time logged)

  Worked               0m
  Breaks               0m
  Average working day  -
  Counted to midnight  0m
  Days without @end    none
`)
	r := e.mustRun("report", "--month", "2026-10")
	assertInOrder(t, "report --month 2026-10", r.stdout,
		"Activity in October 2026\n", "Time logged\n",
		"  Worked               0m\n  Average week         0m\n  Average working day  -\n",
		"  Uncategorised        0m\n  Days without @end    none\n",
		"  2026-10-26 to 2026-10-31     6      0m      0m\n",
		"  Sunday                 0m     4\n")
	if strings.Contains(r.stdout, "Category") {
		t.Errorf("a month without time has no category table:\n%s", r.stdout)
	}
}

func TestReportPeriods_DefaultToThisWeekAndThisMonth(t *testing.T) {
	// Without a date: this week, this month, so far; a period to come
	// says so.
	e, loc := timelogEnv(t)
	now := atNow(t, loc)
	monday := now.AddDate(0, 0, -(int(now.Weekday())+6)%7)
	sunday := monday.AddDate(0, 0, 6)
	soFar := int(now.Sub(time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, loc)).Hours())/24 + 1
	assertInOrder(t, "report --week", e.mustRun("report", "--week").stdout, fmt.Sprintf(
		"Activity in the week %s to %s (so far: %d of 7 days)\n\nCompleted", monday.Format("2006-01-02"), sunday.Format("2006-01-02"), soFar))
	days := time.Date(now.Year(), now.Month()+1, 0, 12, 0, 0, 0, loc).Day()
	assertInOrder(t, "report --month", e.mustRun("report", "--month").stdout, fmt.Sprintf(
		"Activity in %s (so far: %d of %d days)\n\nCompleted", now.Format("January 2006"), now.Day(), days))

	next := monday.AddDate(0, 0, 7).Format("2006-01-02")
	assertInOrder(t, "report --week "+next, e.mustRun("report", "--week", next).stdout,
		"Activity in the week "+next+" to "+monday.AddDate(0, 0, 13).Format("2006-01-02")+" (to come)\n")
}

func TestReportPeriods_FlagsAndTheirErrors(t *testing.T) {
	e, _ := timelogEnv(t)
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--week", "--month"}, "--week and --month exclude each other"},
		{[]string{"--week", "--since", "7d"}, "--week and --since exclude each other"},
		{[]string{"--month", "2026-10", "--since", "2026-10-01"}, "--month and --since exclude each other"},
		{[]string{"--since", "7d", "--week", "--month"}, "--week, --month and --since exclude each other"},
		{[]string{"2026-11-02"}, `"2026-11-02": htt report takes a date only after --week (YYYY-MM-DD) or --month (YYYY-MM)`},
		{[]string{"--since", "7d", "2026-11-02"}, `"2026-11-02": htt report takes a date only after --week (YYYY-MM-DD) or --month (YYYY-MM)`},
		{[]string{"--week", "2026-11-02", "2026-11-09"}, "--week takes one date, got 2: 2026-11-02 2026-11-09"},
		{[]string{"--week", "2026-11-31"}, `--week: want a date, YYYY-MM-DD, got "2026-11-31"`},
		{[]string{"--week", "2026-11"}, `--week: want a date, YYYY-MM-DD, got "2026-11"`},
		{[]string{"--week", "monday"}, `--week: want a date, YYYY-MM-DD, got "monday"`},
		{[]string{"--month", "2026-13"}, `--month: want a month, YYYY-MM, or a date in it, YYYY-MM-DD, got "2026-13"`},
		{[]string{"--month", "October"}, `--month: want a month, YYYY-MM, or a date in it, YYYY-MM-DD, got "October"`},
	}
	for _, c := range cases {
		r := e.run(append([]string{"report"}, c.args...)...)
		if r.exitCode != 1 {
			t.Errorf("report %s: exit %d, want 1", strings.Join(c.args, " "), r.exitCode)
		}
		assertContains(t, "report "+strings.Join(c.args, " ")+": stderr", r.stderr, c.want)
		assertEqual(t, "report "+strings.Join(c.args, " ")+": stdout", r.stdout, "")
	}
}

func TestReportPeriods_StopBeforeTheyPrint(t *testing.T) {
	// A config htt can't use, or a log in the period it can't read:
	// the error, naming the key or the day, and nothing on stdout.
	e, loc := timelogEnv(t)
	e.writeTimelog(loc, "2026-11-02", "09:00 work", "17:00 @end")
	e.writeData(timelogFile("2026-11-03"), "an entry without its time\n")
	for _, args := range [][]string{{"--week", "2026-11-02"}, {"--month", "2026-11"}} {
		r := e.run(append([]string{"report"}, args...)...)
		if r.exitCode != 1 || r.stdout != "" {
			t.Errorf("report %s: exit %d, stdout %q; want 1 and nothing", strings.Join(args, " "), r.exitCode, r.stdout)
		}
		assertContains(t, "report "+strings.Join(args, " ")+": stderr", r.stderr, "read the timelog for 2026-11-03: ")
	}

	e.writeConfig("timelog:\n  default_category: misc\n")
	for _, args := range [][]string{{"--week"}, {"--month", "2026-10"}} {
		r := e.run(append([]string{"report"}, args...)...)
		if r.exitCode != 1 || r.stdout != "" {
			t.Errorf("report %s: exit %d, stdout %q; want 1 and nothing", strings.Join(args, " "), r.exitCode, r.stdout)
		}
		assertContains(t, "report "+strings.Join(args, " ")+": stderr", r.stderr,
			`config: timelog.default_category: "misc" isn't a category`)
	}
}

func TestReportPeriods_InTheREPL(t *testing.T) {
	// The REPL resets the flags between commands: a week, then the
	// default, then --since and a month, neither refused for the
	// flag before.
	e, loc := timelogEnv(t)
	e.writeConfig(planConfig)
	e.writePeriodData(loc)
	r := e.mustRunWithInput("/report --week 2026-11-04\n/report\n/report --since 2w\n/report --month 2026-10\nq\n", "repl")
	assertEqual(t, "stderr", r.stderr, "")
	screens := replScreens(t, r.stdout)
	if len(screens) != 5 {
		t.Fatalf("want 5 screens, got %d:\n%s", len(screens), r.stdout)
	}
	assertContains(t, "--week", screens[1], "Activity in the week 2026-11-02 to 2026-11-08\n")
	assertContains(t, "the default", screens[2], "Activity since ")
	assertContains(t, "--since 2w", screens[3], "Activity since ")
	assertContains(t, "--month", screens[4], "Activity in October 2026\n")
}

// The cross-check: htt report and htt gui, the real binary, on the same
// logs at the same time, print the same figures. The report's text is
// read into rows of fields (split at two or more spaces); the GUI's
// figures are put into the same rows, formatted as the report formats
// them. A figure either gets wrong differs.

// crossView is the part of the GUI's API answer the report prints.
type crossView struct {
	Categories []struct {
		Name          string
		Work          bool
		Uncategorised bool
		CountedAs     string
	}
	Period struct {
		Start, End    string
		IncludesToday bool
	}
	Days    []crossDay
	Summary struct {
		Days               int
		Totals             crossTotals
		AverageWeek        *int64
		AverageWorkingDay  *int64
		UncategorisedShare *float64
		Unended            []string
	}
	Weeks []struct {
		Start, End string
		Days       int
		Totals     crossTotals
	}
	Weekdays []struct {
		Weekday string
		Days    int
		Average *crossTotals
	}
	Halves []struct {
		Start, End string
		Days       int
	}
	Table []struct {
		Category      string
		Total         int64
		ShareOfWorked *float64
		PerWeek       [2]*int64
		Change        *float64
	}
}

type crossDay struct {
	Date, Status string
	crossTotals
}

type crossTotals struct {
	Worked, Breaks, ToMidnight int64
	Categories                 map[string]int64
}

// reportDuration is seconds as the report prints a duration: hours and
// minutes, the minutes rounded down.
func reportDuration(seconds int64) string {
	m := seconds / 60
	if m/60 == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %dm", m/60, m%60)
}

func orDash[T any](v *T, format func(T) string) string {
	if v == nil {
		return "-"
	}
	return format(*v)
}

func percent(x float64) string { return fmt.Sprintf("%d%%", int(math.Round(x*100))) }

func change(x float64) string {
	if x > 0 {
		return "+" + percent(x)
	}
	return percent(x)
}

var fieldSeparator = regexp.MustCompile(`\s{2,}`)

// reportBlocks reads the report's heading and its time section: the
// section's blocks (separated by blank lines), each line split into its
// fields.
func reportBlocks(t *testing.T, stdout string) (string, [][][]string) {
	t.Helper()
	heading, _, _ := strings.Cut(stdout, "\n")
	var blocks [][][]string
	var block [][]string
	for _, line := range strings.Split(strings.TrimPrefix(timeSection(t, stdout), "Time logged\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			if block != nil {
				blocks = append(blocks, block)
			}
			block = nil
			continue
		}
		block = append(block, fieldSeparator.Split(strings.TrimSpace(line), -1))
	}
	return heading, blocks
}

// periodHeading is the heading the report gives the GUI's period.
func periodHeading(name string, v crossView) string {
	switch {
	case v.Period.IncludesToday:
		return fmt.Sprintf("Activity in %s (so far: %d of %d days)", name, v.Summary.Days, len(v.Days))
	case v.Summary.Days == 0:
		return "Activity in " + name + " (to come)"
	}
	return "Activity in " + name
}

// categoryRows are the lines of time per category in t, as the report
// prints them under a day or the total.
func categoryRows(v crossView, t crossTotals) [][]string {
	var rows [][]string
	for _, c := range v.Categories {
		secs := t.Categories[c.Name]
		switch {
		case secs == 0:
		case c.Uncategorised:
			rows = append(rows, []string{c.Name, reportDuration(secs), "(counted as " + c.CountedAs + ")"})
		default:
			rows = append(rows, []string{c.Name, reportDuration(secs)})
		}
	}
	return rows
}

// weekBlocks is the week's time section, from the GUI's figures.
func weekBlocks(v crossView) (string, [][][]string) {
	var days [][]string
	for _, d := range v.Days {
		if d.Status == "future" || d.Worked+d.Breaks == 0 && d.Status != "inProgress" {
			continue
		}
		note := map[string]string{"unended": " (no @end: counted to 24:00)", "inProgress": " (in progress)"}[d.Status]
		days = append(days, []string{d.Date, reportDuration(d.Worked) + " worked" + note})
		if d.Status == "unended" {
			days = append(days, []string{`end the day with htt log end --at "` + d.Date + ` HH:MM"`})
		}
		days = append(days, categoryRows(v, d.crossTotals)...)
	}
	s := v.Summary
	if days == nil {
		days = [][]string{{"(no time logged)"}}
	} else {
		days = append(days, []string{"Total: " + reportDuration(s.Totals.Worked) + " worked"})
		days = append(days, categoryRows(v, s.Totals)...)
	}
	unended := "none"
	if len(s.Unended) > 0 {
		unended = strings.Join(s.Unended, ", ")
	}
	return periodHeading("the week "+v.Period.Start+" to "+v.Period.End, v), [][][]string{days, {
		{"Worked", reportDuration(s.Totals.Worked)},
		{"Breaks", reportDuration(s.Totals.Breaks)},
		{"Average working day", orDash(s.AverageWorkingDay, reportDuration)},
		{"Counted to midnight", reportDuration(s.Totals.ToMidnight)},
		{"Days without @end", unended},
	}}
}

// monthBlocks is the month's time section, from the GUI's figures.
func monthBlocks(t *testing.T, v crossView) (string, [][][]string) {
	t.Helper()
	s := v.Summary
	unended := "none"
	if len(s.Unended) > 0 {
		unended = strings.Join(s.Unended, ", ")
	}
	uncategorised := ""
	for _, c := range v.Categories {
		if c.Uncategorised {
			uncategorised = reportDuration(s.Totals.Categories[c.Name])
			if s.UncategorisedShare != nil {
				uncategorised = percent(*s.UncategorisedShare) + " of worked"
			} else if !c.Work {
				uncategorised += " (counted as " + c.CountedAs + ")"
			}
		}
	}
	blocks := [][][]string{{
		{"Worked", reportDuration(s.Totals.Worked)},
		{"Average week", orDash(s.AverageWeek, reportDuration)},
		{"Average working day", orDash(s.AverageWorkingDay, reportDuration)},
		{"Breaks", reportDuration(s.Totals.Breaks)},
		{"Uncategorised", uncategorised},
		{"Days without @end", unended},
	}}
	weeks := [][]string{{"Week", "Days", "Worked", "Breaks"}}
	for _, w := range v.Weeks {
		weeks = append(weeks, []string{w.Start + " to " + w.End, strconv.Itoa(w.Days), reportDuration(w.Totals.Worked), reportDuration(w.Totals.Breaks)})
	}
	weekdays := [][]string{{"Weekday", "Average worked", "Days"}}
	names := map[string]string{"Mon": "Monday", "Tue": "Tuesday", "Wed": "Wednesday", "Thu": "Thursday", "Fri": "Friday", "Sat": "Saturday", "Sun": "Sunday"}
	for _, wd := range v.Weekdays {
		avg := "-"
		if wd.Average != nil {
			avg = reportDuration(wd.Average.Worked)
		}
		weekdays = append(weekdays, []string{names[wd.Weekday], avg, strconv.Itoa(wd.Days)})
	}
	blocks = append(blocks, weeks, weekdays)

	day := func(date string) int {
		d, err := time.Parse("2006-01-02", date)
		if err != nil {
			t.Fatal(err)
		}
		return d.Day()
	}
	labels := [2]string{"1-15", fmt.Sprintf("16-%d", day(v.Period.End))}
	for i, h := range v.Halves {
		if h.Days > 0 {
			labels[i] = fmt.Sprintf("%d-%d", day(h.Start), day(h.End))
		}
	}
	table := [][]string{{"Category", "Total", "Of worked", "Per week " + labels[0], "Per week " + labels[1], "Change"}}
	for _, row := range v.Table {
		if row.Total == 0 {
			continue
		}
		table = append(table, []string{row.Category, reportDuration(row.Total), orDash(row.ShareOfWorked, percent),
			orDash(row.PerWeek[0], reportDuration), orDash(row.PerWeek[1], reportDuration), orDash(row.Change, change)})
	}
	if len(table) > 1 {
		blocks = append(blocks, table)
	}
	start, err := time.Parse("2006-01-02", v.Period.Start)
	if err != nil {
		t.Fatal(err)
	}
	return periodHeading(start.Format("January 2006"), v), blocks
}

// crossCheck runs `htt report args…` between two fetches of the GUI's
// api, at the same logs, and compares the report's heading and time
// section with the GUI's figures. Today's entry may be running: if
// the two fetches don't give the same figures (a minute turned between
// them), it tries again; the figures only grow with the time, so the
// report's, between them, are theirs.
func crossCheck(t *testing.T, e *env, g *guiProcess, api string, args ...string) {
	t.Helper()
	fromGUI := func() (string, [][][]string, crossView) {
		code, body := fetch(t, http.MethodGet, g.url+api, "")
		var v crossView
		if err := json.Unmarshal(body, &v); err != nil || code != http.StatusOK {
			t.Fatalf("GET %s: %d %v\n%s", api, code, err, body)
		}
		if strings.HasPrefix(api, "api/week") {
			h, b := weekBlocks(v)
			return h, b, v
		}
		h, b := monthBlocks(t, v)
		return h, b, v
	}
	for try := 1; ; try++ {
		wantHeading, want, v := fromGUI()
		r := e.mustRun(append([]string{"report"}, args...)...)
		afterHeading, after, _ := fromGUI()
		if afterHeading != wantHeading || !reflect.DeepEqual(after, want) {
			if try < 5 {
				continue
			}
			t.Fatalf("htt gui's figures changed between two fetches, %d times running", try)
		}
		heading, got := reportBlocks(t, r.stdout)
		if heading != wantHeading {
			t.Errorf("report %s: heading %q, want %q from htt gui's %s", strings.Join(args, " "), heading, wantHeading, api)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("report %s and htt gui's %s differ:\n%s\nthe GUI's figures: %+v", strings.Join(args, " "), api, diffBlocks(got, want), v.Summary)
		}
		if r.stderr != "" {
			t.Errorf("report %s: stderr %s", strings.Join(args, " "), r.stderr)
		}
		return
	}
}

// diffBlocks lists the rows that differ, the report's then the GUI's.
func diffBlocks(got, want [][][]string) string {
	var b strings.Builder
	for i := range max(len(got), len(want)) {
		var g, w [][]string
		if i < len(got) {
			g = got[i]
		}
		if i < len(want) {
			w = want[i]
		}
		for j := range max(len(g), len(w)) {
			var gr, wr []string
			if j < len(g) {
				gr = g[j]
			}
			if j < len(w) {
				wr = w[j]
			}
			if !reflect.DeepEqual(gr, wr) {
				fmt.Fprintf(&b, "  block %d, line %d:\n    report: %q\n    gui:    %q\n", i+1, j+1, gr, wr)
			}
		}
	}
	return b.String()
}

func TestReportPeriods_TheSameFiguresAsTheGUI(t *testing.T) {
	e, loc := timelogEnv(t)
	e.writeConfig(planConfig + "check_for_updates: false\n")
	e.writePeriodData(loc)
	// This week and month: two days ago ended, yesterday without @end,
	// today running, and a log for tomorrow written by hand, which
	// neither reads.
	now := atNow(t, loc)
	day := func(n int) string { return now.AddDate(0, 0, n).Format("2006-01-02") }
	e.writeTimelog(loc, day(-2), "08:30 plan +admin", "09:00 fix +htt", "12:00 @lunch", "12:40 read the news", "17:10 @end")
	e.writeTimelog(loc, day(-1), "09:15 review +htt", "11:00 mail @email")
	e.writeTimelog(loc, day(0),
		earlierToday(now, 0.2).Format("15:04")+" fix +htt",
		earlierToday(now, 0.5).Format("15:04")+" @lunch",
		earlierToday(now, 0.6).Format("15:04")+" mail @email")
	e.writeTimelog(loc, day(1), "09:00 by hand +htt", "17:00 @end")
	g := e.startGUI()

	crossCheck(t, e, g, "api/week?date=2026-11-04", "--week", "2026-11-04") // a past week
	crossCheck(t, e, g, "api/month?month=2026-10", "--month", "2026-10")    // a past month, a day without @end
	crossCheck(t, e, g, "api/week", "--week")                               // this week, so far
	crossCheck(t, e, g, "api/month", "--month")                             // this month, so far
	next := now.AddDate(0, 1, 0).Format("2006-01")
	crossCheck(t, e, g, "api/month?month="+next, "--month", next) // a month to come
}
