package e2e

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// log status and htt status look at the most recent day before today
// with a time log, up to 14 days back, and when it has no @end add a
// line saying so, with the command that ends it, after the usual
// status. log active stays one line.

const (
	notWorkingToday = "Not currently working on any task.\n" // log status, nothing logged today
	noActiveTask    = "No active task.\n"                    // log active, the same
)

// unendedLine is the line status adds for date, a past day without an
// @end.
func unendedLine(date string) string {
	return date + ` has no @end (counted to 24:00): end it with htt log end --at "` + date + ` HH:MM"` + "\n"
}

// statusShows runs log status, log active and htt status, and checks
// that log status prints status, htt status starts with it, straight
// before its contexts, log active prints active, and none of them
// prints anything on stderr.
func (e *env) statusShows(label, status, active string) {
	e.t.Helper()
	r := e.mustRun("log", "status")
	assertEqual(e.t, label+": log status", r.stdout, status)
	assertEqual(e.t, label+": log status stderr", r.stderr, "")
	r = e.mustRun("log", "active")
	assertEqual(e.t, label+": log active", r.stdout, active)
	assertEqual(e.t, label+": log active stderr", r.stderr, "")
	r = e.mustRun("status")
	if !strings.HasPrefix(r.stdout, status+"Available Contexts: ") {
		e.t.Errorf("%s: htt status = %q, want it to start with %q, then the contexts", label, r.stdout, status)
	}
	assertEqual(e.t, label+": htt status stderr", r.stderr, "")
}

// daysAgo is the date n days before now.
func daysAgo(now time.Time, n int) string {
	return now.AddDate(0, 0, -n).Format("2006-01-02")
}

func TestStatus_FlagsYesterdayWithoutAnEnd(t *testing.T) {
	e, loc := timelogEnv(t)
	now := atNow(t, loc)
	yesterday := daysAgo(now, 1)
	e.writeTimelog(loc, yesterday, "09:00 @start", "09:30 review the PR +htt")

	// Nothing logged today: the usual line, then the flag.
	e.statusShows("unended yesterday", notWorkingToday+unendedLine(yesterday), noActiveTask)

	// Ended after the fact, as the line says: the flag goes.
	e.mustRun("log", "end", "--at", "yesterday 17:30")
	e.statusShows("ended with --at", notWorkingToday, noActiveTask)
}

func TestStatus_FlagsAfterTodaysStatus(t *testing.T) {
	e, loc := timelogEnv(t)
	now := atNow(t, loc)
	yesterday := daysAgo(now, 1)
	e.writeTimelog(loc, yesterday, "09:00 work")

	// Working today: today's entry, then yesterday's flag.
	e.writeTodayLog(loc, todayEntry{5*time.Minute + 10*time.Second, "review the PR +htt"})
	e.statusShows("working today",
		"Currently working on: review the PR +htt (uncategorised, 5m)\n"+unendedLine(yesterday),
		"Working on: review the PR +htt (uncategorised, 5m)\n")

	// Today ended too.
	start, end := earlierToday(now, 0.25), earlierToday(now, 0.5)
	e.writeTimelog(loc, now.Format("2006-01-02"), start.Format("15:04")+" work", end.Format("15:04")+" @end")
	ended := "Not working (day ended at " + end.Format("15:04") + ")\n"
	e.statusShows("today ended", ended+unendedLine(yesterday), ended)
}

func TestStatus_NothingExtraWhenTheLastDayEnded(t *testing.T) {
	cases := []struct {
		name string
		logs map[int][]string // days ago: the day's entries
	}{
		{"nothing logged", nil},
		{"yesterday ended", map[int][]string{1: {"09:00 work", "17:00 @end"}}},
		// The entries are read in time order, so the @end ends the day
		// wherever it is in the file.
		{"yesterday's entries all before an @end", map[int][]string{1: {"17:00 @end", "09:00 work", "12:00 @lunch"}}},
		// Only the most recent day is checked: an older day without an
		// @end is the report's to flag.
		{"yesterday ended, the day before didn't", map[int][]string{1: {"09:00 work", "17:00 @end"}, 2: {"09:00 work"}}},
		{"the last log 15 days ago", map[int][]string{15: {"09:00 work"}}},
		{"the last log a month ago", map[int][]string{30: {"09:00 work"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, loc := timelogEnv(t)
			now := atNow(t, loc)
			for ago, entries := range c.logs {
				e.writeTimelog(loc, daysAgo(now, ago), entries...)
			}
			e.statusShows(c.name, notWorkingToday, noActiveTask)
		})
	}
}

func TestStatus_FlagsTheLastDayLoggedUpToTwoWeeksBack(t *testing.T) {
	cases := []struct {
		name    string
		logs    map[int][]string // days ago: the day's entries
		flagged int              // days ago
	}{
		{"three days ago, after a gap", map[int][]string{3: {"09:00 work"}}, 3},
		{"14 days ago, the furthest back", map[int][]string{14: {"09:00 work"}}, 14},
		{"the most recent of two", map[int][]string{4: {"09:00 work"}, 6: {"09:00 work", "17:00 @end"}}, 4},
		// A log file with no entries in it isn't a day logged.
		{"past a log without entries", map[int][]string{1: {}, 2: {"09:00 work"}}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, loc := timelogEnv(t)
			now := atNow(t, loc)
			for ago, entries := range c.logs {
				e.writeTimelog(loc, daysAgo(now, ago), entries...)
			}
			e.statusShows(c.name, notWorkingToday+unendedLine(daysAgo(now, c.flagged)), noActiveTask)
		})
	}
}

func TestStatus_FlagsWhateverTheTimelogConfig(t *testing.T) {
	// The flag needs no categories: with a config htt can't use, it is
	// there all the same, and status warns as before, only when it
	// shows today's entry.
	e, loc := timelogEnv(t)
	now := atNow(t, loc)
	yesterday := daysAgo(now, 1)
	e.writeConfig("timelog:\n  default_category: misc\n")
	e.writeTimelog(loc, yesterday, "09:00 work")
	e.statusShows("bad config, nothing today", notWorkingToday+unendedLine(yesterday), noActiveTask)

	e.writeTodayLog(loc, todayEntry{5*time.Minute + 10*time.Second, "@lunch"})
	warning := `warning: config: timelog.default_category: "misc" isn't a category (want one of: working); ` +
		"showing entries without their categories\n"
	r := e.mustRun("log", "status")
	assertEqual(t, "log status", r.stdout, "Currently working on: @lunch (5m)\n"+unendedLine(yesterday))
	assertEqual(t, "log status stderr", r.stderr, warning)
	r = e.mustRun("log", "active")
	assertEqual(t, "log active", r.stdout, "Working on: @lunch (5m)\n")
	assertEqual(t, "log active stderr", r.stderr, warning)
}

func TestStatus_ThePastDaysDatesDontWarn(t *testing.T) {
	// Status reads the past day only for its @end: a date htt can't
	// read in its entries is the report's to warn about.
	e, loc := timelogEnv(t)
	now := atNow(t, loc)
	yesterday := daysAgo(now, 1)
	e.writeTimelog(loc, yesterday, "09:00 call the bank about 2026-13-45")
	e.statusShows("a bad date yesterday", notWorkingToday+unendedLine(yesterday), noActiveTask)
	file := filepath.Join(e.dataDir, timelogFile(yesterday))
	assertEqual(t, "report stderr", e.mustRun("report").stderr,
		"warning: "+file+":1: 2026-13-45 isn't a real date, so it's text\n")
}

func TestStatus_APastLogItCantReadWarnsAndCarriesOn(t *testing.T) {
	// A past day's log htt can't read doesn't stop today's status: it
	// warns on stderr, adds no line, and the exit code is unchanged.
	e, loc := timelogEnv(t)
	now := atNow(t, loc)
	yesterday := daysAgo(now, 1)
	e.writeData(timelogFile(yesterday), "work without a timestamp\n")
	warning := "warning: read the timelog for " + yesterday +
		`: entry 0: timelog entry missing ts annotation: "work without a timestamp"; not checking that day for an @end` + "\n"
	for _, args := range [][]string{{"log", "status"}, {"status"}} {
		label := "htt " + strings.Join(args, " ")
		r := e.mustRun(args...) // exit 0
		if !strings.HasPrefix(r.stdout, notWorkingToday) || strings.Contains(r.stdout, "@end") {
			t.Errorf("%s = %q, want today's status and no flag", label, r.stdout)
		}
		assertEqual(t, label+": stderr", r.stderr, warning)
	}
	r := e.mustRun("log", "active")
	assertEqual(t, "log active", r.stdout, noActiveTask)
	assertEqual(t, "log active stderr", r.stderr, "")
}

func TestStatus_HelpSaysItFlagsTheLastDay(t *testing.T) {
	e := newEnv(t)
	for _, args := range [][]string{{"log", "status"}, {"status"}} {
		out := e.mustRun(append(args, "--help")...).stdout
		label := "htt " + strings.Join(args, " ") + " --help"
		assertContains(t, label, out, "has no @end")
		assertContains(t, label, out, `htt log end --at "YYYY-MM-DD HH:MM"`)
	}
	if out := e.mustRun("log", "active", "--help").stdout; strings.Contains(out, "has no @end") {
		t.Errorf("htt log active --help mentions the flag, which it doesn't print:\n%s", out)
	}
}
