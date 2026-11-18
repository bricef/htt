package e2e

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// atNow is now in loc, for tests that log --at times relative to it. It
// skips the test within ten minutes of midnight there, where today and
// yesterday could change between the test and htt, and the day so far
// is too short for the times the tests take from it (earlierToday).
func atNow(t *testing.T, loc *time.Location) time.Time {
	t.Helper()
	now := time.Now().In(loc)
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	if now.Sub(midnight) < 10*time.Minute || midnight.AddDate(0, 0, 1).Sub(now) < 10*time.Minute {
		t.Skipf("too close to midnight in %s", loc)
	}
	return now
}

// earlierToday is the time frac (0 to 1) of the way from midnight to
// now, to the minute: a time earlier today, whenever the test runs.
func earlierToday(now time.Time, frac float64) time.Time {
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return midnight.Add(time.Duration(float64(now.Sub(midnight)) * frac)).Truncate(time.Minute)
}

// stamp is the ts: htt gives an entry logged at at.
func stamp(at time.Time) string {
	return "ts:" + at.UTC().Format(time.RFC3339)
}

func timelogFile(date string) string {
	return filepath.Join("timelogs", date+".log")
}

func TestLogAt_ISO8601(t *testing.T) {
	// --at takes ISO 8601: local time without a zone, that instant
	// with one (so a ts: copied from the log works), and seconds when
	// given, which the confirmation then shows. timelogZone is +05:30.
	e, loc := timelogEnv(t)
	now := atNow(t, loc)
	d := now.AddDate(0, 0, -1)
	day := d.Format("2006-01-02")
	at := func(hh, mm, ss int) time.Time {
		return time.Date(d.Year(), d.Month(), d.Day(), hh, mm, ss, 0, loc)
	}
	nine, copied, late := at(9, 0, 0), at(10, 30, 15), at(22, 30, 0)
	for _, c := range []struct{ at, entry, confirm string }{
		{day + "T09:00", "local", "Logging entry: local (at 09:00 yesterday)\n"},
		{copied.UTC().Format(time.RFC3339), "copied", "Logging entry: copied (at 10:30:15 yesterday)\n"},
		{day + "T12:00-05:00", "offset", "Logging entry: offset (at 22:30 yesterday)\n"},
	} {
		r := e.mustRun("log", "add", "--at", c.at, c.entry)
		assertEqual(t, "--at "+c.at, r.stdout, c.confirm)
	}
	assertEqual(t, "yesterday's log", e.readData(timelogFile(day)),
		"local "+stamp(nine)+"\ncopied "+stamp(copied)+"\noffset "+stamp(late)+"\n")
}

func TestLogAt_FixesAForgottenEnd(t *testing.T) {
	// The day was never ended: it runs to midnight, and the report
	// says how to end it. log end --at ends it after the fact.
	e, loc := timelogEnv(t)
	e.writeConfig(planConfig)
	now := atNow(t, loc)
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	e.writeTimelog(loc, yesterday, "09:00 @start", "09:30 review the PR +htt", "13:00 mail @email")
	before := e.readData(timelogFile(yesterday))

	r := e.mustRun("report", "--since", yesterday)
	assertEqual(t, "time section before", timeSection(t, r.stdout), `Time logged
  `+yesterday+`  15h 0m worked (no @end: counted to 24:00)
              end the day with htt log end --at "`+yesterday+` HH:MM"
    htt            3h 30m
    admin          11h 0m
    uncategorised     30m  (counted as working)
  Total: 15h 0m worked
    htt            3h 30m
    admin          11h 0m
    uncategorised     30m  (counted as working)
`)

	r = e.mustRun("log", "end", "--at", "yesterday 17:30")
	assertEqual(t, "log end --at", r.stdout, "Logging entry: @end (at 17:30 yesterday)\n")
	assertEqual(t, "log end --at: stderr", r.stderr, "")
	end, err := time.ParseInLocation("2006-01-02 15:04", yesterday+" 17:30", loc)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "yesterday's log", e.readData(timelogFile(yesterday)), before+"@end "+stamp(end)+"\n")
	if e.dataExists(timelogFile(now.Format("2006-01-02"))) {
		t.Errorf("log end --at yesterday wrote today's log")
	}

	r = e.mustRun("report", "--since", yesterday)
	assertEqual(t, "time section after", timeSection(t, r.stdout), `Time logged
  `+yesterday+`  8h 30m worked
    htt            3h 30m
    admin          4h 30m
    uncategorised     30m  (counted as working)
  Total: 8h 30m worked
    htt            3h 30m
    admin          4h 30m
    uncategorised     30m  (counted as working)
`)
}

func TestReport_OnlyAnUnendedDayGetsTheFix(t *testing.T) {
	// Not an ended day, not today in progress: the day without an @end.
	e, loc := timelogEnv(t)
	now := atNow(t, loc)
	today, yesterday := now.Format("2006-01-02"), now.AddDate(0, 0, -1).Format("2006-01-02")
	e.writeTimelog(loc, "2026-11-02", "09:00 work", "17:00 @end")
	e.writeTimelog(loc, yesterday, "09:00 work")
	e.writeTimelog(loc, today, earlierToday(now, 0.5).Format("15:04")+" work")
	out := timeSection(t, e.mustRun("report", "--since", "2026-11-02").stdout)
	fix := "\n              end the day with htt log end --at \"" + yesterday + " HH:MM\"\n"
	if strings.Count(out, "end the day with") != 1 || !strings.Contains(out, "(no @end: counted to 24:00)"+fix) {
		t.Errorf("time section: want the fix once, under %s's line:\n%s", yesterday, out)
	}
	assertContains(t, "time section", out, " 8h 0m worked\n    uncategorised")
	assertContains(t, "time section", out, " worked (in progress)\n    uncategorised")
}

func TestLogAt_ADateEndsThatDay(t *testing.T) {
	// The report's command, as it gives it, with the time filled in.
	e, loc := timelogEnv(t)
	e.writeTimelog(loc, "2026-11-04", "09:00 @start", "09:30 work")
	r := e.mustRun("log", "end", "--at", "2026-11-04 17:00")
	assertEqual(t, "log end --at a date", r.stdout, "Logging entry: @end (at 17:00 on 2026-11-04)\n")
	r = e.mustRun("report", "--since", "2026-11-04")
	assertEqual(t, "time section", timeSection(t, r.stdout), `Time logged
  2026-11-04  8h 0m worked
    uncategorised  8h 0m  (counted as working)
  Total: 8h 0m worked
    uncategorised  8h 0m  (counted as working)
`)
}

func TestLogAt_LateStart(t *testing.T) {
	// Working since earlier, logged only now: the @start goes before
	// the entry logged since, at the top of the day.
	e, loc := timelogEnv(t)
	now := atNow(t, loc)
	today := now.Format("2006-01-02")
	e.mustRun("log", "add", "review the PR +htt")
	started := earlierToday(now, 0.5)

	r := e.mustRun("log", "start", "--at", started.Format("15:04"))
	assertEqual(t, "log start --at", r.stdout, "Logging entry: @start (at "+started.Format("15:04")+" today)\n")
	show := e.mustRun("log", "show").stdout
	lines := strings.Split(strings.TrimSuffix(show, "\n"), "\n")
	if len(lines) != 2 || lines[0] != "@start "+stamp(started) || !strings.HasPrefix(lines[1], "review the PR +htt ts:") {
		t.Errorf("log show = %q, want @start %s, then the review", lines, stamp(started))
	}
	assertEqual(t, "today's log", e.readData(timelogFile(today)), show)
	// The entry logged now is still the one running.
	assertContains(t, "log status", e.mustRun("log", "status").stdout, "Currently working on: review the PR +htt (uncategorised, ")
}

func TestLogAt_AnEntryMidDayShowsInOrder(t *testing.T) {
	e, loc := timelogEnv(t)
	now := atNow(t, loc)
	today := now.Format("2006-01-02")
	first, middle, last := earlierToday(now, 0.25), earlierToday(now, 0.5), earlierToday(now, 0.75)
	e.writeTimelog(loc, today, first.Format("15:04")+" @start", last.Format("15:04")+" writing the report")

	r := e.mustRun("log", "add", "--at", middle.Format("15:04"), "standup", "@meeting")
	assertEqual(t, "log add --at", r.stdout, "Logging entry: standup @meeting (at "+middle.Format("15:04")+" today)\n")
	want := "@start " + stamp(first) + "\n" +
		"standup @meeting " + stamp(middle) + "\n" +
		"writing the report " + stamp(last) + "\n"
	assertEqual(t, "log show", e.mustRun("log", "show").stdout, want)
	assertEqual(t, "today's log", e.readData(timelogFile(today)), want)
}

func TestLogAt_Workon(t *testing.T) {
	e, loc := timelogEnv(t)
	now := atNow(t, loc)
	today := now.Format("2006-01-02")
	e.mustRun("todo", "add", "write the report +work")
	started := earlierToday(now, 0.5)

	r := e.mustRun("workon", "0", "--at", started.Format("15:04"))
	assertEqual(t, "workon --at", r.stdout, "Logging entry: "+today+" write the report +work (at "+started.Format("15:04")+" today)\n")
	assertEqual(t, "today's log", e.readData(timelogFile(today)), today+" write the report +work "+stamp(started)+"\n")

	r = e.mustRun("wo", "0", "--at", "2026-11-10 08:15")
	assertEqual(t, "wo --at a date", r.stdout, "Logging entry: "+today+" write the report +work (at 08:15 on 2026-11-10)\n")
	assertEqual(t, "2026-11-10's log", e.readData(timelogFile("2026-11-10")),
		today+" write the report +work ts:2026-11-10T02:45:00Z\n")
	// The task itself is as it was.
	assertContains(t, "todo show", e.mustRun("todo", "show").stdout, "  0 "+today+" write the report +work\n")
}

func TestLogAt_ConfirmationSaysWhen(t *testing.T) {
	// Each command that logs names the time --at gave it, by its day:
	// today, yesterday or a date, whichever form named it.
	e, loc := timelogEnv(t)
	now := atNow(t, loc)
	today, yesterday := now.Format("2006-01-02"), now.AddDate(0, 0, -1).Format("2006-01-02")
	e.mustRun("todo", "add", "write the report +work")
	cases := []struct {
		args []string
		want string
	}{
		// (atNow keeps these earlier than now.)
		{[]string{"log", "start", "--at", "0:01"}, "Logging entry: @start (at 00:01 today)\n"},
		{[]string{"log", "add", "--at", "00:02", "reading", "mail"}, "Logging entry: reading mail (at 00:02 today)\n"},
		{[]string{"log", "+", "--at=" + today + " 00:03", "filing"}, "Logging entry: filing (at 00:03 today)\n"},
		{[]string{"workon", "0", "--at", "yesterday 9:00"}, "Logging entry: " + today + " write the report +work (at 09:00 yesterday)\n"},
		{[]string{"log", "end", "--at", yesterday + " 18:00"}, "Logging entry: @end (at 18:00 yesterday)\n"},
		{[]string{"log", "add", "--at", "2026-01-31 23:59", "late"}, "Logging entry: late (at 23:59 on 2026-01-31)\n"},
	}
	for _, c := range cases {
		r := e.mustRun(c.args...)
		assertEqual(t, "htt "+strings.Join(c.args, " "), r.stdout, c.want)
		assertEqual(t, "htt "+strings.Join(c.args, " ")+": stderr", r.stderr, "")
	}
	assertEqual(t, "yesterday's log", e.readData(timelogFile(yesterday)),
		today+" write the report +work ts:"+yesterday+"T03:30:00Z\n@end ts:"+yesterday+"T12:30:00Z\n")
	assertEqual(t, "2026-01-31's log", e.readData(timelogFile("2026-01-31")), "late ts:2026-01-31T18:29:00Z\n")
}

func TestLogAt_LaterThanNowIsRefused(t *testing.T) {
	// Refused by every command that logs, with nothing written: not
	// today's log, not the day it names, not a backup.
	e, loc := timelogEnv(t)
	now := atNow(t, loc)
	today, tomorrow := now.Format("2006-01-02"), now.AddDate(0, 0, 1).Format("2006-01-02")
	e.mustRun("todo", "add", "write the report +work")
	e.writeTimelog(loc, today, earlierToday(now, 0.5).Format("15:04")+" @start")
	before := e.readData(timelogFile(today))

	soon := now.Add(5 * time.Minute).Format("15:04") // atNow keeps it today
	soonUTC := now.Add(5 * time.Minute).UTC().Format(time.RFC3339)
	for _, at := range []string{soon, "23:59", tomorrow + " 09:00", "2099-01-01 00:00", tomorrow + "T09:00", soonUTC} {
		for _, args := range [][]string{
			{"log", "start"}, {"log", "add", "something"}, {"log", "end"}, {"workon", "0"},
		} {
			args = append(args, "--at", at)
			label := "htt " + strings.Join(args, " ")
			r := e.run(args...)
			if r.exitCode != 1 {
				t.Errorf("%s exited %d, want 1", label, r.exitCode)
			}
			assertEqual(t, label+": stdout", r.stdout, "")
			if !strings.HasPrefix(r.stderr, fmt.Sprintf("❌ --at %q is later than now (", at)) ||
				!strings.HasSuffix(r.stderr, "): nothing logged\n") {
				t.Errorf("%s: stderr = %q, want ❌ --at %q is later than now (HH:MM): nothing logged", label, r.stderr, at)
			}
		}
	}
	assertEqual(t, "today's log", e.readData(timelogFile(today)), before)
	for _, rel := range []string{timelogFile(today) + ".bak", timelogFile(tomorrow), timelogFile("2099-01-01")} {
		if e.dataExists(rel) {
			t.Errorf("a refused --at wrote %s", rel)
		}
	}
}

func TestLogAt_ATimeHttCantReadIsAnError(t *testing.T) {
	e, loc := timelogEnv(t)
	atNow(t, loc)
	for _, at := range []string{"9.30", "25:00", "2026-11-17", "yesterday", "5pm", "Yesterday 17:30", ""} {
		args := []string{"log", "end", "--at", at}
		r := e.run(args...)
		label := fmt.Sprintf("htt log end --at %q", at)
		if r.exitCode != 1 {
			t.Errorf("%s exited %d, want 1", label, r.exitCode)
		}
		assertEqual(t, label+": stdout", r.stdout, "")
		assertEqual(t, label+": stderr", r.stderr, fmt.Sprintf("❌ --at: %q isn't a time htt can read "+
			`(use HH:MM for today, "yesterday HH:MM" or "YYYY-MM-DD HH:MM", on the 24-hour clock, `+
			`or ISO 8601's YYYY-MM-DDTHH:MM, with :SS and a zone (Z, +05:30) if you like)`+"\n", at))
	}
	if e.dataExists("timelogs") {
		t.Errorf("a bad --at wrote a time log")
	}
}

func TestLogAt_ATypedEntryWarnsNamingItsLine(t *testing.T) {
	// A date htt can't read in the entry warns as log add does without
	// --at, naming the day's file and the line the entry went to.
	e, loc := timelogEnv(t)
	now := atNow(t, loc)
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	e.writeTimelog(loc, yesterday, "09:00 @start", "12:00 @lunch")
	r := e.mustRun("log", "add", "--at", "yesterday 10:00", "read", "the", "2026-5-3", "minutes")
	assertEqual(t, "stdout", r.stdout, "Logging entry: read the 2026-5-3 minutes (at 10:00 yesterday)\n")
	assertEqual(t, "stderr", r.stderr, "warning: "+filepath.Join(e.dataDir, timelogFile(yesterday))+
		":2: 2026-5-3 isn't written YYYY-MM-DD (2026-05-03), so it's text\n")
}

func TestLogAt_TheDayTheClocksChange(t *testing.T) {
	// A time the clocks skip is what Go's time.Date makes of it, and the
	// confirmation shows it. In London, 01:30 on the day the clocks go
	// forward is 02:30; in Santiago, whose clocks go forward at
	// midnight, 00:30 is 23:30 the day before, and is logged there.
	cases := []struct {
		zone, at, confirm, file, entry string
	}{
		{"Europe/London", "2026-03-29 01:30", "(at 02:30 on 2026-03-29)", "2026-03-29", "night shift ts:2026-03-29T01:30:00Z\n"},
		{"Europe/London", "2026-10-25 01:30", "(at 01:30 on 2026-10-25)", "2026-10-25", "night shift ts:2026-10-25T01:30:00Z\n"},
		{"America/Santiago", "2026-09-06 00:30", "(at 23:30 on 2026-09-05)", "2026-09-05", "night shift ts:2026-09-06T03:30:00Z\n"},
	}
	for _, c := range cases {
		e := newEnv(t)
		e.extraEnv = append(e.extraEnv, "TZ="+c.zone)
		r := e.mustRun("log", "add", "--at", c.at, "night", "shift")
		assertEqual(t, c.zone+" "+c.at, r.stdout, "Logging entry: night shift "+c.confirm+"\n")
		assertEqual(t, c.zone+" "+c.at+": the log", e.readData(timelogFile(c.file)), c.entry)
	}
}

func TestRepl_At(t *testing.T) {
	// The REPL reads quotes as the shell does, so every --at form
	// works there: HH:MM, a quoted "yesterday 17:30", and --at='…'.
	// The next command's entry is logged now: --at doesn't carry over.
	e, loc := timelogEnv(t)
	now := atNow(t, loc)
	today, yesterday := now.Format("2006-01-02"), now.AddDate(0, 0, -1).Format("2006-01-02")
	r := e.mustRunWithInput("/log add --at 0:01 first\n/log end --at \"yesterday 17:30\"\n"+
		"/log add --at='yesterday 9:00' filing\n/log add second\nq\n", "repl")
	assertContains(t, "stdout", r.stdout, "Logging entry: first (at 00:01 today)\n")
	assertContains(t, "stdout", r.stdout, "Logging entry: @end (at 17:30 yesterday)\n")
	assertContains(t, "stdout", r.stdout, "Logging entry: filing (at 09:00 yesterday)\n")
	assertContains(t, "stdout", r.stdout, "Logging entry: second\n")
	assertEqual(t, "stderr", r.stderr, "")
	at := func(day, clock string) time.Time {
		t.Helper()
		when, err := time.ParseInLocation("2006-01-02 15:04", day+" "+clock, loc)
		if err != nil {
			t.Fatal(err)
		}
		return when
	}
	first := at(today, "00:01")
	lines := strings.Split(e.readData(timelogFile(today)), "\n")
	if len(lines) != 3 || lines[0] != "first "+stamp(first) || !strings.HasPrefix(lines[1], "second ts:") || lines[1] == "second "+stamp(first) {
		t.Errorf("today's log = %q, want first at 00:01, then second now", lines)
	}
	assertEqual(t, "yesterday's log", e.readData(timelogFile(yesterday)),
		"filing "+stamp(at(yesterday, "09:00"))+"\n@end "+stamp(at(yesterday, "17:30"))+"\n")
}

func TestRepl_QuotesGroupWordsAndApostrophesStay(t *testing.T) {
	// Quotes in a REPL line group words and are dropped, as in the
	// shell; an apostrophe inside a word is kept as typed.
	e := newEnv(t)
	e.mustRunWithInput("add \"buy milk\" for Sam's party\nq\n", "repl")
	assertEqual(t, "todo.txt", e.readData("todo.txt"), today()+" buy milk for Sam's party\n")
}

func TestLogAt_HelpNamesTheForms(t *testing.T) {
	e := newEnv(t)
	for _, args := range [][]string{{"log", "start"}, {"log", "add"}, {"log", "end"}, {"workon"}} {
		out := e.mustRun(append(args, "--help")...).stdout
		label := "htt " + strings.Join(args, " ") + " --help"
		assertContains(t, label, out, `--at time   log it at HH:MM (today), "yesterday HH:MM", "YYYY-MM-DD HH:MM" or YYYY-MM-DDTHH:MM`+"\n")
		assertContains(t, label, out, "so a ts: copied from htt\nlog show works.")
		assertContains(t, label, out, "A time later than now is refused,\nand nothing is logged.")
	}
	assertContains(t, "htt log end --help", e.mustRun("log", "end", "--help").stdout, `htt log end --at "yesterday 17:30"`)
}
