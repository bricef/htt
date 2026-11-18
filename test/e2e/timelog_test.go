package e2e

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // the test's own copy of the zone htt runs in
)

// The time-log tests run htt in Kolkata (UTC+05:30, no clock changes),
// so a day's local midnight isn't UTC's: htt stamps entries in UTC and
// must count each day from local midnight to local midnight.
const timelogZone = "Asia/Kolkata"

func timelogEnv(t *testing.T) (*env, *time.Location) {
	t.Helper()
	loc, err := time.LoadLocation(timelogZone)
	if err != nil {
		t.Fatal(err)
	}
	e := newEnv(t)
	e.extraEnv = append(e.extraEnv, "TZ="+timelogZone)
	return e, loc
}

// writeTimelog writes the log file for date from lines "HH:MM text",
// each an entry at that local time, stamped in UTC as htt stamps them.
func (e *env) writeTimelog(loc *time.Location, date string, lines ...string) {
	e.t.Helper()
	var b strings.Builder
	for _, line := range lines {
		clock, text, _ := strings.Cut(line, " ")
		at, err := time.ParseInLocation("2006-01-02 15:04", date+" "+clock, loc)
		if err != nil {
			e.t.Fatal(err)
		}
		fmt.Fprintf(&b, "%s ts:%s\n", text, at.UTC().Format(time.RFC3339))
	}
	e.writeData(filepath.Join("timelogs", date+".log"), b.String())
}

// writeTodayLog writes today's log from entries logged the given time
// before now, oldest first. It skips the test when today (in loc)
// hasn't lasted long enough to hold them.
func (e *env) writeTodayLog(loc *time.Location, entries ...todayEntry) {
	e.t.Helper()
	now := time.Now().In(loc)
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	var b strings.Builder
	for _, entry := range entries {
		at := now.Add(-entry.ago)
		if !at.After(midnight.Add(time.Minute)) || entry.ago > now.Sub(midnight)-2*time.Minute {
			e.t.Skipf("too close to midnight in %s for entries %v ago", timelogZone, entry.ago)
		}
		fmt.Fprintf(&b, "%s ts:%s\n", entry.text, at.UTC().Format(time.RFC3339))
	}
	e.writeData(filepath.Join("timelogs", now.Format("2006-01-02")+".log"), b.String())
}

type todayEntry struct {
	ago  time.Duration
	text string
}

// timeSection is the report's time section, the last.
func timeSection(t *testing.T, stdout string) string {
	t.Helper()
	i := strings.Index(stdout, "Time logged\n")
	if i < 0 {
		t.Fatalf("no time section in the report:\n%s", stdout)
	}
	return stdout[i:]
}

// planConfig is the plan's example categories.
const planConfig = `timelog:
  categories:            # first match wins
    - name: break
      tags: ["@break", "@lunch"]
      work: false
    - name: htt
      tags: ["+htt"]
    - name: admin
      tags: ["@email", "+admin"]
  default_category: working
`

func TestReport_TimeByCategoryPerDay(t *testing.T) {
	e, loc := timelogEnv(t)
	e.writeConfig(planConfig)
	// The headline day: lunch is an entry, @end ends the day.
	e.writeTimelog(loc, "2026-11-02",
		"09:00 @start", "09:00 writing the report", "12:00 @lunch", "13:00 writing the report", "17:00 @end")
	// The same day as people logged it before: @end for lunch. The gap
	// after it counts as nothing, and there's no break.
	e.writeTimelog(loc, "2026-11-03",
		"09:00 @start", "09:00 writing the report", "12:00 @end", "13:00 writing the report", "17:00 @end")
	// No @end: the last entry runs to 24:00, and the day is flagged.
	e.writeTimelog(loc, "2026-11-04",
		"09:00 @start", "09:30 review the PR +htt", "13:00 mail @email")

	r := e.mustRun("report", "--since", "2026-11-02")
	assertEqual(t, "time section", timeSection(t, r.stdout), `Time logged
  2026-11-02   7h 0m worked
    break      1h 0m
    working    7h 0m
  2026-11-03   7h 0m worked
    working    7h 0m
  2026-11-04  15h 0m worked (no @end: counted to 24:00)
    htt       3h 30m
    admin     11h 0m
    working      30m
  Total: 29h 0m worked
    break      1h 0m
    htt       3h 30m
    admin     11h 0m
    working  14h 30m
`)
	assertEqual(t, "stderr", r.stderr, "")
}

func TestReport_TimeWithoutCategoriesIsAllWorking(t *testing.T) {
	// With no categories configured, a break entry is just another
	// entry: everything is working.
	e, loc := timelogEnv(t)
	e.writeTimelog(loc, "2026-11-02", "09:00 work", "12:00 @lunch", "13:00 work", "17:00 @end")
	r := e.mustRun("report", "--since", "2026-11-02")
	assertEqual(t, "time section", timeSection(t, r.stdout), `Time logged
  2026-11-02  8h 0m worked
    working  8h 0m
  Total: 8h 0m worked
    working  8h 0m
`)
}

func TestReport_TheDayTheClocksGoBackIsOneDay(t *testing.T) {
	// In London, 2026-10-25 lasts 25 hours: a report stepping 24 hours
	// from its midnight lands on the same date again.
	e := newEnv(t)
	e.extraEnv = append(e.extraEnv, "TZ=Europe/London")
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Fatal(err)
	}
	for _, date := range []string{"2026-10-24", "2026-10-25", "2026-10-26"} {
		e.writeTimelog(loc, date, "09:00 work", "10:00 @end")
	}
	r := e.mustRun("report", "--since", "2026-10-24")
	assertEqual(t, "time section", timeSection(t, r.stdout), `Time logged
  2026-10-24  1h 0m worked
    working  1h 0m
  2026-10-25  1h 0m worked
    working  1h 0m
  2026-10-26  1h 0m worked
    working  1h 0m
  Total: 3h 0m worked
    working  3h 0m
`)
}

func TestReport_NoTimeLogged(t *testing.T) {
	e, _ := timelogEnv(t)
	r := e.mustRun("report")
	assertEqual(t, "time section", timeSection(t, r.stdout), "Time logged\n  (no time logged)\n")
}

func TestReport_TodaysOpenEntryRunsUntilNow(t *testing.T) {
	// It counts until now, marked in progress, rather than being left
	// out ("excluding currently-active entry").
	e, loc := timelogEnv(t)
	e.writeConfig(planConfig)
	e.writeTodayLog(loc,
		todayEntry{90*time.Minute + 30*time.Second, "@start"},
		todayEntry{60*time.Minute + 30*time.Second, "review the PR +htt"})
	r := e.mustRun("report")
	today := time.Now().In(loc).Format("2006-01-02")
	assertEqual(t, "time section", timeSection(t, r.stdout), `Time logged
  `+today+`  1h 30m worked (in progress)
    htt      1h 0m
    working    30m
  Total: 1h 30m worked
    htt      1h 0m
    working    30m
`)
}

func TestReport_BadTimelogConfigFailsNamingTheKey(t *testing.T) {
	e, loc := timelogEnv(t)
	e.writeTimelog(loc, "2026-11-02", "09:00 work", "17:00 @end")
	cases := []struct{ config, want string }{
		{"timelog:\n  categories:\n    - name: admin\n",
			"config: timelog.categories: category 1 (admin): no tags, and it isn't the default category, so no entry can be in it; list its tags, or make it the default category"},
		{"timelog:\n  default_category: misc\n",
			`config: timelog.default_category: "misc" isn't a category (want one of: working)`},
	}
	for _, tc := range cases {
		e.writeConfig(tc.config)
		r := e.run("report", "--since", "2026-11-02")
		if r.exitCode == 0 {
			t.Errorf("report with %q: want a non-zero exit", tc.config)
		}
		assertContains(t, "stderr", r.stderr, tc.want)
		// It fails before the report starts.
		assertEqual(t, "stdout", r.stdout, "")

		// Commands that don't need the categories carry on.
		e.mustRun("log", "add", "still logging")
		e.mustRun("log", "status")
		e.mustRun("todo", "add", "still works")
	}
}

func TestLog_StatusAndActiveFollowTheDay(t *testing.T) {
	e, loc := timelogEnv(t)

	// Nothing logged today.
	assertEqual(t, "log status, empty", e.mustRun("log", "status").stdout, "Not currently working on any task.\n")
	assertEqual(t, "log active, empty", e.mustRun("log", "active").stdout, "No active task.\n")

	// Working: the current entry and how long it has run.
	e.writeTodayLog(loc,
		todayEntry{2*time.Hour + 30*time.Second, "@start"},
		todayEntry{time.Hour + 30*time.Second, "review the PR +htt"})
	assertEqual(t, "log status, working", e.mustRun("log", "status").stdout,
		"Currently working on: review the PR +htt (1h0m)\n")
	assertEqual(t, "log active, working", e.mustRun("log", "active").stdout,
		"Working on: review the PR +htt (1h0m)\n")
	assertInOrder(t, "status, working", e.mustRun("status").stdout,
		"Currently working on: review the PR +htt (1h0m)\n", "Available Contexts:")

	// After @end: not working, and since when.
	ended := 20*time.Minute + 30*time.Second
	e.writeTodayLog(loc,
		todayEntry{2*time.Hour + 30*time.Second, "@start"},
		todayEntry{time.Hour + 30*time.Second, "review the PR +htt"},
		todayEntry{ended, "@end"})
	at := time.Now().In(loc).Add(-ended).Format("15:04")
	want := "Not working (day ended at " + at + ")\n"
	assertEqual(t, "log status, ended", e.mustRun("log", "status").stdout, want)
	assertEqual(t, "log active, ended", e.mustRun("log", "active").stdout, want)
	assertInOrder(t, "status, ended", e.mustRun("status").stdout, want, "Available Contexts:")

	// An entry after @end starts again.
	e.mustRun("log", "add", "back at it")
	out := e.mustRun("log", "status").stdout
	if !strings.HasPrefix(out, "Currently working on: back at it (") {
		t.Errorf("log status after an entry that follows @end = %q, want it working on the entry", out)
	}
}

func TestLog_EndThenStatusSaysTheDayEnded(t *testing.T) {
	// The commands themselves, not a written file: log end, then status.
	e, loc := timelogEnv(t)
	e.mustRun("log", "start")
	before := time.Now().In(loc)
	assertEqual(t, "log end", e.mustRun("log", "end").stdout, "Logging entry: @end\n")
	after := time.Now().In(loc)
	out := e.mustRun("log", "status").stdout
	if out != "Not working (day ended at "+before.Format("15:04")+")\n" &&
		out != "Not working (day ended at "+after.Format("15:04")+")\n" {
		t.Errorf("log status after log end = %q, want Not working (day ended at %s)", out, before.Format("15:04"))
	}
}
