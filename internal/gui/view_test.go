package gui

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // the zones below, wherever the tests run

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/storage"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

const testPort = 8123

// testZone is the zone the tests run htt in: London's clocks go back
// on 25 October 2026.
func testZone(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// testNow is the tests' clock: Thursday 19 November 2026, 15:20, and a
// fraction of a second, which the server drops.
func testNow(t *testing.T) time.Time {
	return time.Date(2026, 11, 19, 15, 20, 0, 250_000_000, testZone(t))
}

func testCategories(t *testing.T) *domain.Categoriser {
	t.Helper()
	c, err := domain.NewCategoriser([]domain.Category{
		{Name: "break", Tags: []string{"@break", "@lunch"}, Work: false},
		{Name: "htt", Tags: []string{"+htt"}, Work: true},
		{Name: "admin", Tags: []string{"@email", "+admin"}, Work: true},
		{Name: "meetings", Tags: []string{"@meeting"}, Work: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// testLogs are the time logs the tests read: lines "HH:MM text" by
// date, each an entry at that local time, stamped in UTC as htt
// stamps them; or "<RFC 3339> text", an entry at that instant.
var testLogs = map[string][]string{
	// The week of 16 November; today is the 19th.
	"2026-11-16": {"09:00 planning @meeting", "09:30 Windows build +htt", "12:30 @lunch", "13:15 reading the news", "13:45 Windows build +htt", "17:30 @end"},
	"2026-11-17": {"09:15 answer mail @email", "09:45 status flags +htt"}, // no @end
	"2026-11-19": {"09:05 @start", "09:10 prototype +htt", "12:30 @lunch", "13:15 (A) review the plan +htt due:2026-11-20"},
	"2026-11-20": {"09:00 a log for tomorrow, by hand +htt"}, // not read: it's to come
	// October, and the end of September for its first rolling weeks.
	"2026-09-28": {"09:00 fix +htt", "17:00 @end"},
	"2026-10-01": {"09:00 triage @email", "09:30 parser +htt", "12:00 @lunch", "12:45 parser +htt", "17:00 @end"},
	"2026-10-02": {"10:00 standup @meeting", "10:15 release +htt", "16:00 @end"},
	"2026-10-15": {"09:00 parser +htt"}, // no @end
	"2026-10-16": {"09:00 reading the news", "10:00 @end"},
	// The clocks go back at 02:00: 01:30 BST to 02:30 GMT is two hours,
	// an hour on the wall clock. 01:30 happens twice, so it's given as
	// an instant.
	"2026-10-25": {"2026-10-25T00:30:00Z late fix +htt", "02:30 @end"},
	"2026-10-31": {"10:00 docs +htt", "11:30 @end"},
}

// testRepo is an in-memory repository holding logs.
func testRepo(t *testing.T, loc *time.Location, logs map[string][]string) domain.TimelogRepository {
	t.Helper()
	repo := storage.NewMemoryTimelogRepository()
	for date, lines := range logs {
		day, err := time.ParseInLocation("2006-01-02 15:04", date+" 12:00", loc)
		if err != nil {
			t.Fatal(err)
		}
		l := domain.NewTimelog(repo, day)
		for _, line := range lines {
			clock, text, _ := strings.Cut(line, " ")
			at, err := time.Parse(time.RFC3339, clock)
			if err != nil {
				at, err = time.ParseInLocation("2006-01-02 15:04", date+" "+clock, loc)
			}
			if err != nil {
				t.Fatal(err)
			}
			task, err := domain.NewTask(fmt.Sprintf("%s ts:%s", text, at.UTC().Format(time.RFC3339)))
			if err != nil {
				t.Fatal(err)
			}
			l.Entries = append(l.Entries, task)
		}
		if err := repo.Save(l); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

// testServer serves testLogs with the test categories and clock.
func testServer(t *testing.T) *Server {
	t.Helper()
	return New(Config{
		Timelogs:   testRepo(t, testZone(t), testLogs),
		Categories: testCategories(t),
		Now:        func() time.Time { return testNow(t) },
	})
}

// get asks s's handler for path, as the browser would.
func get(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8123/"+path, nil)
	w := httptest.NewRecorder()
	s.Handler(testPort).ServeHTTP(w, r)
	return w
}

// getView asks for path and reads the view it answers.
func getView(t *testing.T, s *Server, path string) view {
	t.Helper()
	w := get(t, s, path)
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", path, w.Code, w.Body)
	}
	var v view
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("GET %s: %v\n%s", path, err, w.Body)
	}
	return v
}

// checkGolden compares body, indented, with testdata/name, or writes it
// there with -update.
func checkGolden(t *testing.T, name string, body []byte) {
	t.Helper()
	var indented bytes.Buffer
	if err := json.Indent(&indented, body, "", "  "); err != nil {
		t.Fatal(err)
	}
	indented.WriteByte('\n')
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, indented.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run the test with -update to write it)", err)
	}
	if !bytes.Equal(indented.Bytes(), want) {
		got := strings.Split(indented.String(), "\n")
		lines := strings.Split(string(want), "\n")
		for i := range min(len(got), len(lines)) {
			if got[i] != lines[i] {
				t.Fatalf("%s differs at line %d:\n got %s\nwant %s\n(run with -update to accept the new answer)", path, i+1, got[i], lines[i])
			}
		}
		t.Fatalf("%s differs in length: %d lines, want %d", path, len(got), len(lines))
	}
}

func TestWeek_Golden(t *testing.T) {
	w := get(t, testServer(t), "api/week?date=2026-11-18")
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	checkGolden(t, "week-2026-11-16.json", w.Body.Bytes())
}

func TestMonth_Golden(t *testing.T) {
	w := get(t, testServer(t), "api/month?month=2026-10")
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	checkGolden(t, "month-2026-10.json", w.Body.Bytes())
}

func TestWeek_DaysAndTotals(t *testing.T) {
	// The golden file holds everything; this says what matters in it.
	v := getView(t, testServer(t), "api/week?date=2026-11-22") // the Sunday: its week
	if v.Version != apiVersion || v.Now != "2026-11-19T15:20:00Z" || v.NowMinute != 920 || v.Today != "2026-11-19" {
		t.Errorf("version %d, now %s (minute %v), today %s", v.Version, v.Now, v.NowMinute, v.Today)
	}
	wantPeriod := period{Kind: "week", Key: "2026-11-16", Start: "2026-11-16", End: "2026-11-22",
		Previous: "2026-11-09", Next: "2026-11-23", Current: "2026-11-16", IncludesToday: true}
	if v.Period != wantPeriod {
		t.Errorf("period %+v, want %+v", v.Period, wantPeriod)
	}
	var names []string
	for _, c := range v.Categories {
		names = append(names, fmt.Sprintf("%s/%v/%d", c.Name, c.Work, c.Colour))
	}
	if got := strings.Join(names, " "); got != "break/false/0 htt/true/0 admin/true/1 meetings/true/2 uncategorised/true/0" {
		t.Errorf("categories %s", got)
	}
	if last := v.Categories[len(v.Categories)-1]; !last.Uncategorised || last.CountedAs != "working" {
		t.Errorf("uncategorised is %+v", last)
	}

	statuses := map[string]string{}
	for _, d := range v.Days {
		statuses[d.Date] = d.Status
	}
	want := map[string]string{"2026-11-16": "ended", "2026-11-17": "unended", "2026-11-18": "empty",
		"2026-11-19": "inProgress", "2026-11-20": "future", "2026-11-21": "future", "2026-11-22": "future"}
	for date, s := range want {
		if statuses[date] != s {
			t.Errorf("%s is %q, want %q", date, statuses[date], s)
		}
	}
	h := int64(3600)
	mon, tue, thu, fri := v.Days[0], v.Days[1], v.Days[3], v.Days[4]
	// Monday: 30m meeting, 3h + 3h45 htt, 45m lunch, 30m news; @end at 17:30.
	if mon.Worked != 7*h+45*60 || mon.Breaks != 45*60 || mon.Categories["uncategorised"] != 30*60 || mon.EndedAt != "2026-11-16T17:30:00Z" || *mon.EndedAtMinute != 1050 {
		t.Errorf("Monday: %+v", mon)
	}
	if last := tue.Intervals[len(tue.Intervals)-1]; last.Open != "midnight" || last.EndMinute != 1440 || last.End != "2026-11-18T00:00:00Z" || tue.ToMidnight != 14*h+15*60 {
		t.Errorf("Tuesday's last entry: %+v, to midnight %d", last, tue.ToMidnight)
	}
	now := thu.Intervals[len(thu.Intervals)-1]
	if now.Open != "now" || now.EndMinute != 920 || now.Seconds != 2*h+5*60 || now.Entry != "(A) review the plan +htt due:2026-11-20" {
		t.Errorf("today's last entry: %+v", now)
	}
	if fri.Status != "future" || len(fri.Intervals) != 0 || fri.Worked != 0 {
		t.Errorf("tomorrow's log was read: %+v", fri)
	}
	if start := thu.Intervals[0]; start.Category != "working" || !start.Uncategorised || !start.Work || start.StartMinute != 545 {
		t.Errorf("@start is %+v, want uncategorised in working", start)
	}
	if v.Hours == nil || v.Hours.From != 480 || v.Hours.To != 1440 {
		t.Errorf("hours %+v, want 08:00 to 24:00 (a day without @end)", v.Hours)
	}
	s := v.Summary
	if s.Days != 4 || s.WorkingDays != 3 || len(s.Unended) != 1 || s.Unended[0] != "2026-11-17" {
		t.Errorf("summary: %d days, %d working, unended %v", s.Days, s.WorkingDays, s.Unended)
	}
	worked := mon.Worked + tue.Worked + thu.Worked
	if s.Totals.Worked != worked || s.Totals.ToMidnight != tue.ToMidnight || *s.AverageWorkingDay != worked/3 {
		t.Errorf("summary totals %+v, average working day %d; want %d worked", s.Totals, *s.AverageWorkingDay, worked)
	}
	if v.Weeks != nil || v.Rolling != nil || v.Table != nil {
		t.Errorf("a week with the month's figures")
	}
}

func TestMonth_ThisMonthSoFar(t *testing.T) {
	// November, on the 19th: the days after today are to come, and the
	// figures are over the days so far.
	v := getView(t, testServer(t), "api/month")
	if v.Period.Key != "2026-11" || v.Period.Previous != "2026-10" || v.Period.Next != "2026-12" || !v.Period.IncludesToday || v.Period.Current != "2026-11" {
		t.Errorf("period %+v", v.Period)
	}
	if len(v.Days) != 30 || v.Days[18].Status != "inProgress" || v.Days[19].Status != "future" || v.Days[29].Status != "future" {
		t.Errorf("%d days; the 19th %q, the 20th %q", len(v.Days), v.Days[18].Status, v.Days[19].Status)
	}
	if v.Summary.Days != 19 || len(v.Rolling) != 19 || v.Rolling[18].Date != "2026-11-19" {
		t.Errorf("summary over %d days, %d rolling totals, want 19", v.Summary.Days, len(v.Rolling))
	}
	if v.Halves[0].Days != 15 || v.Halves[1].Days != 4 || v.Halves[1].End != "2026-11-19" {
		t.Errorf("halves %+v", v.Halves)
	}
	days := map[string]int{}
	for _, wd := range v.Weekdays {
		days[wd.Weekday] = wd.Days
	}
	if days["Mon"] != 3 || days["Thu"] != 3 || days["Fri"] != 2 || days["Sun"] != 3 || v.Weekdays[0].Weekday != "Mon" {
		t.Errorf("weekdays %v", days)
	}
	var lengths []int
	for _, w := range v.Weeks {
		lengths = append(lengths, w.Days)
	}
	if fmt.Sprint(lengths) != "[1 7 7 7 7 1]" {
		t.Errorf("weeks of %v days, want November 2026's: Sunday 1, four weeks, Monday 30", lengths)
	}
	// The rolling week to the 19th holds Monday to Thursday.
	wantWorked := v.Days[15].Worked + v.Days[16].Worked + v.Days[18].Worked
	if got := v.Rolling[18].Totals.Worked; got != wantWorked {
		t.Errorf("rolling week to the 19th: %d worked, want %d", got, wantWorked)
	}
}

func TestMonth_FiguresAddUp(t *testing.T) {
	v := getView(t, testServer(t), "api/month?month=2026-10")
	h := int64(3600)
	if v.Summary.Days != 31 || v.Summary.WorkingDays != 6 || fmt.Sprint(v.Summary.Unended) != "[2026-10-15]" {
		t.Errorf("summary: %d days, %d working, unended %v", v.Summary.Days, v.Summary.WorkingDays, v.Summary.Unended)
	}
	// 1 Oct 7h15, 2 Oct 6h, 15 Oct 15h, 16 Oct 1h, 25 Oct 2h, 31 Oct 1h30.
	worked := 7*h + 15*60 + 6*h + 15*h + h + 2*h + h + 30*60
	if got := v.Summary.Totals.Worked; got != worked {
		t.Errorf("worked %d, want %d", got, worked)
	}
	if got := *v.Summary.AverageWeek; got != (worked*7+15)/31 {
		t.Errorf("average week %d, want %d", got, worked*7/31)
	}
	if got := *v.Summary.UncategorisedShare; got != float64(h)/float64(worked) {
		t.Errorf("uncategorised share %v", got)
	}
	var weekly int64
	for _, w := range v.Weeks {
		weekly += w.Totals.Worked
	}
	if weekly != worked {
		t.Errorf("the weeks add up to %d, want %d", weekly, worked)
	}
	if late := v.Days[24].Intervals[0]; late.Seconds != 2*h || late.StartMinute != 90 || late.EndMinute != 150 {
		t.Errorf("the night the clocks went back: %+v", late)
	}
	// The rolling week to 1 October reaches back to 28 September.
	if got := v.Rolling[0].Totals.Categories["htt"]; got != 8*h+6*h+45*60 {
		t.Errorf("htt in the 7 days to 1 Oct: %d", got)
	}
	var htt categoryRow
	for _, row := range v.Table {
		if row.Category == "htt" {
			htt = row
		}
		if row.Category == "break" && row.ShareOfWorked != nil {
			t.Errorf("break has a share of worked time")
		}
	}
	// htt per week: 27h30 over the 15 days to the 15th, 46200s; 3h30
	// over the 16 days after, 5512.5s, rounded.
	if *htt.PerWeek[0] != 46200 || *htt.PerWeek[1] != 5513 {
		t.Errorf("htt per week %d and %d", *htt.PerWeek[0], *htt.PerWeek[1])
	}
	if want := float64(*htt.PerWeek[1])/float64(*htt.PerWeek[0]) - 1; htt.Change == nil || *htt.Change-want > 1e-4 || want-*htt.Change > 1e-4 {
		t.Errorf("htt change %v, want about %v", htt.Change, want)
	}
}

func TestAPI_BadDates(t *testing.T) {
	s := testServer(t)
	for _, path := range []string{
		"api/week?date=2026-13-01", "api/week?date=yesterday", "api/week?date=2026-11-1",
		"api/month?month=2026-1", "api/month?month=2026-10-01", "api/month?month=October",
	} {
		w := get(t, s, path)
		var e apiError
		if w.Code != http.StatusBadRequest || json.Unmarshal(w.Body.Bytes(), &e) != nil || e.Error == "" || e.Version != apiVersion {
			t.Errorf("GET %s: %d %s, want 400 and an error", path, w.Code, w.Body)
		}
	}
	if w := get(t, s, "api/year"); w.Code != http.StatusNotFound {
		t.Errorf("GET api/year: %d, want 404", w.Code)
	}
}

func TestWeek_TheHourThatRepeats(t *testing.T) {
	// London's clocks went back at 02:00 BST on 25 October 2026, to
	// 01:00 GMT. An entry from 01:50 BST to 01:10 GMT is 20 minutes long,
	// and its wall clock runs backwards, minute 110 to minute 70: it's
	// drawn for its 20 minutes, to 02:10, and the timeline opens on it.
	s := New(Config{
		Timelogs: testRepo(t, testZone(t), map[string][]string{
			"2026-10-25": {"2026-10-25T00:50:00Z late fix +htt", "2026-10-25T01:10:00Z @end"},
		}),
		Categories: testCategories(t),
		Now:        func() time.Time { return testNow(t) },
	})
	v := getView(t, s, "api/week?date=2026-10-25")
	ivs := v.Days[6].Intervals
	if len(ivs) != 1 {
		t.Fatalf("25 October: %+v", v.Days[6])
	}
	if iv := ivs[0]; iv.StartMinute != 110 || iv.EndMinute != 70 || iv.DrawnEndMinute != 130 || iv.Seconds != 1200 ||
		iv.Start != "2026-10-25T01:50:00+01:00" || iv.End != "2026-10-25T01:10:00Z" {
		t.Errorf("the entry: %+v", iv)
	}
	if v.Hours == nil || v.Hours.From != 60 || v.Hours.To != 180 {
		t.Errorf("hours %+v, want 01:00 to 03:00", v.Hours)
	}

	// Santiago's go back at midnight, to 23:00 the day before: 23:50 to
	// 23:10 that Saturday is drawn to 24:00, no further.
	scl, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	s = New(Config{
		Timelogs: testRepo(t, scl, map[string][]string{
			"2026-04-04": {"2026-04-05T02:50:00Z late fix +htt", "2026-04-05T03:10:00Z @end"},
		}),
		Categories: testCategories(t),
		Now:        func() time.Time { return testNow(t).In(scl) },
	})
	sat := getView(t, s, "api/week?date=2026-04-04").Days[5]
	if len(sat.Intervals) != 1 || sat.Intervals[0].StartMinute != 1430 || sat.Intervals[0].EndMinute != 1390 || sat.Intervals[0].DrawnEndMinute != 1440 {
		t.Errorf("Santiago, 4 April 2026: %+v", sat)
	}
}

func TestAPI_DatesAreCalendarDates(t *testing.T) {
	// In Asunción the clocks went forward at midnight on Monday 1
	// October 1990 and on Sunday 1 October 2023: those midnights never
	// happened, and Go puts a time that doesn't exist on the day before.
	// A date the API is given is a day on the calendar all the same.
	loc, err := time.LoadLocation("America/Asuncion")
	if err != nil {
		t.Fatal(err)
	}
	s := New(Config{
		Timelogs:   testRepo(t, loc, map[string][]string{"2023-10-02": {"09:00 work +htt", "10:00 @end"}}),
		Categories: testCategories(t),
		Now:        func() time.Time { return time.Date(2023, 11, 15, 12, 0, 0, 0, loc) },
	})
	m := getView(t, s, "api/month?month=2023-10")
	want := period{Kind: "month", Key: "2023-10", Start: "2023-10-01", End: "2023-10-31", Previous: "2023-09", Next: "2023-11", Current: "2023-11"}
	if m.Period != want {
		t.Errorf("month 2023-10: period %+v, want %+v", m.Period, want)
	}
	if len(m.Days) != 31 || m.Days[0].Date != "2023-10-01" || m.Days[1].Worked != 3600 {
		t.Errorf("month 2023-10: %d days from %s, the 2nd worked %d", len(m.Days), m.Days[0].Date, m.Days[1].Worked)
	}
	if next := getView(t, s, "api/month?month=2023-09").Period.Next; next != "2023-10" {
		t.Errorf("the month after September 2023 is %s", next)
	}
	w := getView(t, s, "api/week?date=1990-10-01")
	if w.Period.Start != "1990-10-01" || w.Period.End != "1990-10-07" || w.Period.Previous != "1990-09-24" {
		t.Errorf("the week of Monday 1 October 1990: %+v", w.Period)
	}
	if w := getView(t, s, "api/week?date=2023-10-01"); w.Period.Start != "2023-09-25" || w.Days[6].Date != "2023-10-01" {
		t.Errorf("the week of Sunday 1 October 2023: %+v", w.Period)
	}
}

func TestAPI_BadConfig(t *testing.T) {
	// A config htt can't use: the API says so, naming the key, and the
	// page still loads, to show it.
	s := New(Config{
		Timelogs:  testRepo(t, testZone(t), testLogs),
		ConfigErr: errors.New(`config: timelog.categories: category 2 (htt): "htt" isn't a tag (want +project, @context or #tag)`),
		Now:       func() time.Time { return testNow(t) },
	})
	for _, path := range []string{"api/week", "api/month"} {
		w := get(t, s, path)
		var e apiError
		if w.Code != http.StatusInternalServerError || json.Unmarshal(w.Body.Bytes(), &e) != nil ||
			!strings.Contains(e.Error, "timelog.categories: category 2 (htt)") || !strings.Contains(e.Error, "restart htt gui") {
			t.Errorf("GET %s: %d %s", path, w.Code, w.Body)
		}
	}
	if w := get(t, s, ""); w.Code != http.StatusOK {
		t.Errorf("the page: %d", w.Code)
	}
}

func TestAPI_UnreadableLog(t *testing.T) {
	repo := testRepo(t, testZone(t), nil)
	day := time.Date(2026, 11, 17, 12, 0, 0, 0, testZone(t))
	l := domain.NewTimelog(repo, day)
	task, err := domain.NewTask("an entry without its time")
	if err != nil {
		t.Fatal(err)
	}
	l.Entries = append(l.Entries, task)
	if err := repo.Save(l); err != nil {
		t.Fatal(err)
	}
	s := New(Config{Timelogs: repo, Now: func() time.Time { return testNow(t) }})
	w := get(t, s, "api/week")
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "2026-11-17") {
		t.Errorf("%d %s, want 500 naming the day", w.Code, w.Body)
	}
}

func TestAPI_NoCategories(t *testing.T) {
	// Without categories everything is uncategorised, in working.
	s := New(Config{Timelogs: testRepo(t, testZone(t), testLogs), Now: func() time.Time { return testNow(t) }})
	v := getView(t, s, "api/week?date=2026-11-16")
	if len(v.Categories) != 1 || v.Categories[0].Name != "uncategorised" || !v.Categories[0].Work || v.Categories[0].CountedAs != "working" {
		t.Errorf("categories %+v", v.Categories)
	}
	if mon := v.Days[0]; mon.Breaks != 0 || mon.Categories["uncategorised"] != mon.Worked || mon.Worked != 8*3600+30*60 {
		t.Errorf("Monday %+v", mon)
	}
}

func TestOpeningHours(t *testing.T) {
	iv := func(from, to float64) interval { return interval{StartMinute: from, EndMinute: to, DrawnEndMinute: to} }
	cases := []struct {
		days     []day
		from, to float64
	}{
		{nil, 0, 1440},
		{[]day{{Intervals: []interval{iv(535, 1060)}}}, 480, 1140},                                        // 08:55 to 17:40: 08:00 to 19:00
		{[]day{{Intervals: []interval{iv(500, 600)}}, {Intervals: []interval{iv(700, 1050)}}}, 420, 1080}, // 08:20 to 17:30: 07:00 to 18:00
		{[]day{{Intervals: []interval{iv(10, 1440)}}}, 0, 1440},
		{[]day{{Intervals: []interval{iv(540, 1020)}}}, 480, 1080}, // 09:00 to 17:00: 08:00 to 18:00
	}
	for _, c := range cases {
		if got := openingHours(c.days); got.From != c.from || got.To != c.to {
			t.Errorf("openingHours(%v) = %v, want %v to %v", c.days, got, c.from, c.to)
		}
	}
}
