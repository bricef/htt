package gui

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/bricef/htt/internal/domain"
)

// The JSON API, versioned here, in one place. It is internal to htt:
// the page embedded in the same binary is its only client.
//
// GET api/week?date=YYYY-MM-DD is the week, Monday to Sunday, that holds
// the date; GET api/month?month=YYYY-MM is the calendar month. Each
// defaults to today's. Durations are in seconds. Times are RFC 3339
// with the local offset, each with its minutes since local midnight on
// the wall clock (…Minute), for drawing: so a day the clocks change
// draws by its clock, while durations stay true; the end of a day is
// minute 1440. Where the wall clock runs backwards, as the clocks go
// back, an interval is drawn to its drawnEndMinute (see interval). An
// error is {"version", "error"}, with status 400 for a
// bad date, 500 for a config htt can't use or a log it can't read.
const apiVersion = 1

// view is the API's answer: the week's or the month's days and figures.
type view struct {
	Version    int        `json:"version"`
	Now        string     `json:"now"`
	NowMinute  float64    `json:"nowMinute"`
	Today      string     `json:"today"`
	Categories []category `json:"categories"`
	Period     period     `json:"period"`
	Days       []day      `json:"days"`
	Summary    summary    `json:"summary"` // over the days so far

	// The week's.
	Hours *hours `json:"hours,omitempty"`

	// The month's.
	Weeks    []stretch     `json:"weeks,omitempty"`
	Weekdays []weekday     `json:"weekdays,omitempty"`
	Rolling  []rolling     `json:"rolling,omitempty"`
	Halves   []half        `json:"halves,omitempty"`
	Table    []categoryRow `json:"table,omitempty"`
}

// category is one of the categories time can be in, in the config's
// order, then uncategorised, with the colour to draw it in: its place
// in its palette, the work palette or the muted one for categories
// that aren't work. Uncategorised is grey; it counts as work when the
// default category does.
type category struct {
	Name          string `json:"name"`
	Work          bool   `json:"work"`
	Colour        int    `json:"colour"`
	Uncategorised bool   `json:"uncategorised,omitempty"`
	CountedAs     string `json:"countedAs,omitempty"` // uncategorised: the default category
}

// period is the days shown, and the keys of the periods around them,
// as the routes take them: a week's Monday (YYYY-MM-DD), a month
// (YYYY-MM).
type period struct {
	Kind          string `json:"kind"` // week or month
	Key           string `json:"key"`
	Start         string `json:"start"`
	End           string `json:"end"`
	Previous      string `json:"previous"`
	Next          string `json:"next"`
	Current       string `json:"current"` // the key of the period holding today
	IncludesToday bool   `json:"includesToday"`
}

// totals is time added up: worked, breaks (logged time that isn't
// work), the time of days without @end counted to 24:00, and the time
// per category, uncategorised under "uncategorised" (a name no
// category can have). A category without time is left out.
type totals struct {
	Worked     int64            `json:"worked"`
	Breaks     int64            `json:"breaks"`
	ToMidnight int64            `json:"toMidnight"`
	Categories map[string]int64 `json:"categories"`
}

// day is a day's time log: its status (empty, ended, inProgress,
// unended, or future for a day after today, whose log isn't read), its
// totals, the time of its @end, and its intervals.
type day struct {
	Date   string `json:"date"`
	Status string `json:"status"`
	totals
	EndedAt       string     `json:"endedAt,omitempty"`
	EndedAtMinute *float64   `json:"endedAtMinute,omitempty"`
	Intervals     []interval `json:"intervals"`
}

// interval is an entry's time: its text without its ts:, its category
// (the default, for an uncategorised one), whether its time is work,
// and Open: "now" for today's entry still running, "midnight" for the
// last entry of a day without @end, counted to 24:00.
//
// DrawnEndMinute is where the timeline draws it to: EndMinute, unless
// the wall clock runs backwards (the hour that repeats when the clocks
// go back: 01:50 BST to 01:10 GMT is minute 110 to minute 70), when it's
// drawn for its true length from its start, to 24:00 at most.
type interval struct {
	Entry          string  `json:"entry"`
	Category       string  `json:"category"`
	Uncategorised  bool    `json:"uncategorised"`
	Work           bool    `json:"work"`
	Start          string  `json:"start"`
	StartMinute    float64 `json:"startMinute"`
	End            string  `json:"end"`
	EndMinute      float64 `json:"endMinute"`
	DrawnEndMinute float64 `json:"drawnEndMinute"`
	Seconds        int64   `json:"seconds"`
	Open           string  `json:"open,omitempty"`
}

// summary is the period's figures over its days so far: up to today, in
// the period holding it. The averages are null over no days.
type summary struct {
	Days               int      `json:"days"`
	WorkingDays        int      `json:"workingDays"`
	Totals             totals   `json:"totals"`
	AverageWeek        *int64   `json:"averageWeek"`        // worked ÷ days × 7
	AverageWorkingDay  *int64   `json:"averageWorkingDay"`  // worked ÷ days with worked time
	UncategorisedShare *float64 `json:"uncategorisedShare"` // of worked time; null when it isn't worked time, or none was
	Unended            []string `json:"unended"`            // the days without @end
}

// hours is the stretch of the day the week's timeline opens on: from
// the whole hour at least half an hour before its first entry to the
// whole hour at least half an hour after its last end as drawn (24:00
// for a day without @end), in minutes; the whole day when nothing is
// logged.
type hours struct {
	From float64 `json:"from"`
	To   float64 `json:"to"`
}

// stretch is a run of the month's days: a Monday-to-Sunday week, cut
// at the month's edges.
type stretch struct {
	Start  string `json:"start"`
	End    string `json:"end"`
	Days   int    `json:"days"`
	Totals totals `json:"totals"`
}

// half is the 1st to the 15th or the 16th to the end, so far, and its
// time per week (totals ÷ days × 7); a half without days yet has none.
type half struct {
	stretch
	PerWeek *totals `json:"perWeek"`
}

// weekday is a weekday's time over the month so far, Monday first: its
// days, their total, and the average day (null without days).
type weekday struct {
	Weekday string  `json:"weekday"`
	Days    int     `json:"days"`
	Totals  totals  `json:"totals"`
	Average *totals `json:"average"`
}

// rolling is the time of the 7 days up to a date, for each day of the
// month so far: per category, hours per week. The first days' reach
// into the month before.
type rolling struct {
	Date   string `json:"date"`
	Totals totals `json:"totals"`
}

// categoryRow is a category's line in the month's table: its time,
// its share of worked time (null for a category that isn't work), its
// time per week in each half (null for a half without days yet), and
// the change from the first to the second (null without both, or when
// the first has none).
type categoryRow struct {
	Category      string    `json:"category"`
	Total         int64     `json:"total"`
	ShareOfWorked *float64  `json:"shareOfWorked"`
	PerWeek       [2]*int64 `json:"perWeek"`
	Change        *float64  `json:"change"`
}

// apiError is the API's answer when it has none.
type apiError struct {
	Version int    `json:"version"`
	Error   string `json:"error"`
}

const dateLayout = "2006-01-02"
const monthLayout = "2006-01"

func (s *Server) handleWeek(w http.ResponseWriter, r *http.Request) {
	now := s.now()
	date := now
	if q := r.URL.Query().Get("date"); q != "" {
		d, err := calendarDate(dateLayout, q, now.Location())
		if err != nil {
			writeError(w, r, http.StatusBadRequest, fmt.Errorf("date: want YYYY-MM-DD, got %q", q))
			return
		}
		date = d
	}
	v, err := s.weekView(date, now)
	s.answer(w, r, v, err)
}

func (s *Server) handleMonth(w http.ResponseWriter, r *http.Request) {
	now := s.now()
	date := now
	if q := r.URL.Query().Get("month"); q != "" {
		d, err := calendarDate(monthLayout, q, now.Location())
		if err != nil {
			writeError(w, r, http.StatusBadRequest, fmt.Errorf("month: want YYYY-MM, got %q", q))
			return
		}
		date = d
	}
	v, err := s.monthView(date, now)
	s.answer(w, r, v, err)
}

// calendarDate reads s, a date in layout (a month's is its 1st), as a day
// on the calendar: that day at noon in loc. It isn't read at midnight in
// loc: where the clocks go forward at midnight, that midnight doesn't
// exist, and Go puts it on the day before.
func calendarDate(layout, s string, loc *time.Location) (time.Time, error) {
	d, err := time.Parse(layout, s) // in UTC, where every midnight happens
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, loc), nil
}

// now is the server's clock to the second: entries are logged to the
// second, so every duration is whole seconds.
func (s *Server) now() time.Time {
	return s.cfg.Now().Truncate(time.Second)
}

func (s *Server) answer(w http.ResponseWriter, r *http.Request, v *view, err error) {
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, r, http.StatusOK, v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, err error) {
	writeJSON(w, r, status, apiError{Version: apiVersion, Error: err.Error()})
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Length", fmt.Sprint(len(body)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

// sheets reads each date's log as a timesheet at now. A date after
// today is a day to come: its log isn't read, and it has no time.
func (s *Server) sheets(dates []time.Time, now time.Time) ([]*domain.Timesheet, error) {
	today := noon(now)
	out := make([]*domain.Timesheet, len(dates))
	for i, date := range dates {
		l := &domain.Timelog{Date: date}
		if !date.After(today) {
			var err error
			if l, err = s.cfg.Timelogs.Day(date); err != nil {
				return nil, fmt.Errorf("load the timelog for %s: %w", date.Format(dateLayout), err)
			}
		}
		sheet, err := l.Timesheet(s.cfg.Categories, now)
		if err != nil {
			return nil, fmt.Errorf("read the timelog for %s: %w", date.Format(dateLayout), err)
		}
		out[i] = sheet
	}
	return out, nil
}

func noon(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, t.Location())
}

// soFar is how many of the days are today or before.
func soFar(days []*domain.Timesheet, now time.Time) int {
	today := noon(now)
	n := 0
	for n < len(days) && !days[n].Date.After(today) {
		n++
	}
	return n
}

// base is what every view carries: the version, the clock, and the
// categories; or the config's error.
func (s *Server) base(now time.Time) (*view, error) {
	if s.cfg.ConfigErr != nil {
		return nil, fmt.Errorf("%w; fix it, then restart htt gui", s.cfg.ConfigErr)
	}
	return &view{
		Version:    apiVersion,
		Now:        now.Format(time.RFC3339),
		NowMinute:  wallMinutes(now),
		Today:      now.Format(dateLayout),
		Categories: categoriesOf(s.cfg.Categories),
	}, nil
}

// categoriesOf lists the categories an entry's tags can put it in, in
// the config's order, then uncategorised. A category without tags
// (the built-in default, or the config's default when it has none) has
// no time of its own, only uncategorised time, so it isn't listed.
func categoriesOf(c *domain.Categoriser) []category {
	var out []category
	work, other := 0, 0
	for _, cat := range c.Categories() {
		if len(cat.Tags) == 0 {
			continue
		}
		next := &other
		if cat.Work {
			next = &work
		}
		out = append(out, category{Name: cat.Name, Work: cat.Work, Colour: *next})
		*next++
	}
	def := c.Default()
	return append(out, category{Name: domain.UncategorisedName, Work: def.Work, Uncategorised: true, CountedAs: def.Name})
}

func (s *Server) weekView(date, now time.Time) (*view, error) {
	v, err := s.base(now)
	if err != nil {
		return nil, err
	}
	p := domain.WeekOf(date)
	days, err := s.sheets(p.Dates(), now)
	if err != nil {
		return nil, err
	}
	v.Period = period{
		Kind:          "week",
		Key:           p.First.Format(dateLayout),
		Start:         p.First.Format(dateLayout),
		End:           p.Last.Format(dateLayout),
		Previous:      p.First.AddDate(0, 0, -7).Format(dateLayout),
		Next:          p.First.AddDate(0, 0, 7).Format(dateLayout),
		Current:       domain.WeekOf(now).First.Format(dateLayout),
		IncludesToday: p.Contains(now),
	}
	v.Days = dayViews(days, now)
	v.Summary = summaryOf(domain.Summarise(days[:soFar(days, now)]), s.cfg.Categories.Default())
	h := openingHours(v.Days)
	v.Hours = &h
	return v, nil
}

func (s *Server) monthView(date, now time.Time) (*view, error) {
	v, err := s.base(now)
	if err != nil {
		return nil, err
	}
	p := domain.MonthOf(date)
	// The six days before the month, for its first days' rolling weeks.
	dates := domain.Period{First: p.First.AddDate(0, 0, -6), Last: p.Last}.Dates()
	all, err := s.sheets(dates, now)
	if err != nil {
		return nil, err
	}
	days := all[6:]
	n := soFar(days, now)
	v.Period = period{
		Kind:          "month",
		Key:           p.First.Format(monthLayout),
		Start:         p.First.Format(dateLayout),
		End:           p.Last.Format(dateLayout),
		Previous:      p.First.AddDate(0, -1, 0).Format(monthLayout),
		Next:          p.First.AddDate(0, 1, 0).Format(monthLayout),
		Current:       now.Format(monthLayout),
		IncludesToday: p.Contains(now),
	}
	v.Days = dayViews(days, now)
	sum := domain.Summarise(days[:n])
	v.Summary = summaryOf(sum, s.cfg.Categories.Default())

	for _, w := range domain.Weeks(days) {
		v.Weeks = append(v.Weeks, stretchOf(w))
	}
	for _, a := range domain.WeekdayAverages(days[:n]) {
		wd := weekday{Weekday: a.Weekday.String()[:3], Days: a.Days, Totals: totalsOf(a.Totals)}
		if a.Days > 0 {
			avg := totalsOf(a.Average)
			wd.Average = &avg
		}
		v.Weekdays = append(v.Weekdays, wd)
	}
	for i, t := range domain.Rolling(all[:6+n], 7) {
		v.Rolling = append(v.Rolling, rolling{Date: days[i].Date.Format(dateLayout), Totals: totalsOf(t)})
	}
	first, second := domain.Halves(days[:n])
	for _, h := range []domain.Stretch{first, second} {
		hv := half{stretch: stretchOf(h)}
		if h.Days > 0 {
			pw := totalsOf(h.Totals.Scale(7, h.Days))
			hv.PerWeek = &pw
		}
		v.Halves = append(v.Halves, hv)
	}
	v.Table = tableOf(v.Categories, sum, first, second)
	return v, nil
}

// tableOf is the month's table: a line per category, in the
// categories' order.
func tableOf(categories []category, sum domain.Summary, first, second domain.Stretch) []categoryRow {
	timeIn := func(t domain.Totals, c category) time.Duration {
		if c.Uncategorised {
			return t.Uncategorised
		}
		return t.ByCategory[c.Name]
	}
	var rows []categoryRow
	for _, c := range categories {
		row := categoryRow{Category: c.Name, Total: seconds(timeIn(sum.Totals, c))}
		if c.Work && sum.Totals.Worked > 0 {
			share := float64(timeIn(sum.Totals, c)) / float64(sum.Totals.Worked)
			row.ShareOfWorked = &share
		}
		var rates [2]time.Duration
		for i, h := range []domain.Stretch{first, second} {
			if h.Days > 0 {
				rates[i] = timeIn(h.Totals.Scale(7, h.Days), c)
				secs := seconds(rates[i])
				row.PerWeek[i] = &secs
			}
		}
		if first.Days > 0 && second.Days > 0 && rates[0] > 0 {
			change := float64(rates[1])/float64(rates[0]) - 1
			row.Change = &change
		}
		rows = append(rows, row)
	}
	return rows
}

func dayViews(days []*domain.Timesheet, now time.Time) []day {
	today := noon(now)
	out := make([]day, len(days))
	for i, s := range days {
		d := day{Date: s.Date.Format(dateLayout), totals: totalsOf(s.Totals()), Intervals: []interval{}}
		switch {
		case s.Date.After(today):
			d.Status = "future"
		case s.Status == domain.DayEmpty:
			d.Status = "empty"
		case s.Status == domain.DayEnded:
			d.Status = "ended"
			d.EndedAt = s.EndedAt.In(now.Location()).Format(time.RFC3339)
			m := dayMinutes(s.EndedAt, s)
			d.EndedAtMinute = &m
		case s.Status == domain.DayInProgress:
			d.Status = "inProgress"
		default:
			d.Status = "unended"
		}
		for j, iv := range s.Intervals {
			cat := iv.Category.Name
			// The log was read for this answer alone, so its entries are
			// ours to change.
			entry := iv.Entry.RemoveAnnotation(domain.TimelogTimestampLabel).Raw
			ivv := interval{
				Entry:         entry,
				Category:      cat,
				Uncategorised: iv.Uncategorised,
				Work:          iv.Category.Work,
				Start:         iv.Start.In(now.Location()).Format(time.RFC3339),
				StartMinute:   dayMinutes(iv.Start, s),
				End:           iv.End.In(now.Location()).Format(time.RFC3339),
				EndMinute:     dayMinutes(iv.End, s),
				Seconds:       seconds(iv.Duration()),
			}
			ivv.DrawnEndMinute = ivv.EndMinute
			if ivv.EndMinute < ivv.StartMinute { // the wall clock ran backwards
				ivv.DrawnEndMinute = math.Min(24*60, ivv.StartMinute+iv.Duration().Minutes())
			}
			if j == len(s.Intervals)-1 {
				switch s.Status {
				case domain.DayInProgress:
					ivv.Open = "now"
				case domain.DayUnended:
					ivv.Open = "midnight"
				}
			}
			d.Intervals = append(d.Intervals, ivv)
		}
		out[i] = d
	}
	return out
}

// dayMinutes is t's minutes since the day's midnight on the wall clock,
// 0 at the day's start and 1440 at its end, whatever the day's length.
func dayMinutes(t time.Time, s *domain.Timesheet) float64 {
	switch {
	case !t.After(s.Start):
		return 0
	case !t.Before(s.End):
		return 24 * 60
	}
	return wallMinutes(t.In(s.Start.Location()))
}

// wallMinutes is t's minutes since midnight on its wall clock.
func wallMinutes(t time.Time) float64 {
	return float64(t.Hour()*60+t.Minute()) + float64(t.Second())/60
}

// openingHours is the stretch of the day the timeline opens on; see
// hours.
func openingHours(days []day) hours {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, d := range days {
		for _, iv := range d.Intervals {
			lo = math.Min(lo, iv.StartMinute)
			hi = math.Max(hi, iv.DrawnEndMinute)
		}
	}
	if math.IsInf(lo, 1) {
		return hours{From: 0, To: 24 * 60}
	}
	return hours{
		From: math.Max(0, math.Floor((lo-30)/60)*60),
		To:   math.Min(24*60, math.Ceil((hi+30)/60)*60),
	}
}

func summaryOf(s domain.Summary, def domain.Category) summary {
	out := summary{
		Days:        s.Days,
		WorkingDays: s.WorkingDays,
		Totals:      totalsOf(s.Totals),
		Unended:     []string{},
	}
	if s.Days > 0 {
		avg := seconds(s.AverageWeek)
		out.AverageWeek = &avg
	}
	if s.WorkingDays > 0 {
		avg := seconds(s.AverageWorkingDay)
		out.AverageWorkingDay = &avg
	}
	if def.Work && s.Totals.Worked > 0 {
		share := float64(s.Totals.Uncategorised) / float64(s.Totals.Worked)
		out.UncategorisedShare = &share
	}
	for _, d := range s.Unended {
		out.Unended = append(out.Unended, d.Format(dateLayout))
	}
	return out
}

func stretchOf(s domain.Stretch) stretch {
	out := stretch{Days: s.Days, Totals: totalsOf(s.Totals)}
	if s.Days > 0 {
		out.Start, out.End = s.First.Format(dateLayout), s.Last.Format(dateLayout)
	}
	return out
}

func totalsOf(t domain.Totals) totals {
	out := totals{
		Worked:     seconds(t.Worked),
		Breaks:     seconds(t.Breaks),
		ToMidnight: seconds(t.ToMidnight),
		Categories: map[string]int64{},
	}
	for name, d := range t.ByCategory {
		if d > 0 {
			out.Categories[name] = seconds(d)
		}
	}
	if t.Uncategorised > 0 {
		out.Categories[domain.UncategorisedName] = seconds(t.Uncategorised)
	}
	return out
}

// seconds is d in whole seconds, rounded.
func seconds(d time.Duration) int64 {
	return int64(d.Round(time.Second) / time.Second)
}
