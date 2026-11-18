package domain

import (
	"cmp"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
)

// genDay is one generated day: a time log, the clock, and what the
// test knows about them independently of Timesheet.
type genDay struct {
	log   *Timelog
	now   time.Time
	start time.Time // local midnight
	end   time.Time // the next local midnight
	today bool
}

// generateDay writes a random day's time log: up to nine entries at
// whole minutes, a quarter of them @end, with tags from the test
// categories and near misses, in file order that is mostly but not
// always time order; now and then one from just before or after the
// day, as a hand-edited file or a change of time zone gives; with or
// without a final @end. Four in ten days are today, the clock at a
// random minute of it; the rest are past days.
func generateDay(t *testing.T, r *rand.Rand, zones []*time.Location) genDay {
	t.Helper()
	loc := zones[r.IntN(len(zones))]
	// Days around the zones' clock changes, and any day of the year.
	dates := []string{"2026-03-08", "2026-03-29", "2026-04-05", "2026-10-04", "2026-10-25", "2026-11-01"}
	var date time.Time
	if r.IntN(3) == 0 {
		date, _ = time.ParseInLocation("2006-01-02", dates[r.IntN(len(dates))], loc)
	} else {
		date = time.Date(2026, 1, 1+r.IntN(365), 0, 0, 0, 0, loc)
	}
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)
	minutes := int(end.Sub(start) / time.Minute)

	g := genDay{log: &Timelog{Date: start}, start: start, end: end}
	if r.IntN(10) < 4 {
		g.today = true
		g.now = start.Add(time.Duration(r.IntN(minutes)) * time.Minute)
	} else {
		g.now = end.Add(time.Duration(1+r.IntN(5*24*60)) * time.Minute)
	}

	words := []string{"work", "+htt", "+admin", "@lunch", "@break", "@email", "#deep", "@Break", "@breakfast", "fix the @end bug"}
	var stamps []time.Time
	for range r.IntN(10) {
		at := start.Add(time.Duration(r.IntN(minutes)) * time.Minute)
		if r.IntN(20) == 0 {
			at = start.Add(time.Duration(r.IntN(120)-60) * time.Minute) // around midnight before
		} else if r.IntN(20) == 0 {
			at = end.Add(time.Duration(r.IntN(120)-60) * time.Minute) // around midnight after
		}
		stamps = append(stamps, at)
	}
	slices.SortFunc(stamps, func(a, b time.Time) int { return a.Compare(b) })
	if len(stamps) > 1 && r.IntN(5) == 0 { // a hand edit put two out of order
		i, j := r.IntN(len(stamps)), r.IntN(len(stamps))
		stamps[i], stamps[j] = stamps[j], stamps[i]
	}
	for _, at := range stamps {
		text := DayEndEntry
		if r.IntN(4) != 0 {
			text = "doing"
			for range r.IntN(3) {
				text += " " + words[r.IntN(len(words))]
			}
		}
		g.log.Entries = append(g.log.Entries, timestampedEntry(t, text, at.UTC()))
	}
	if len(stamps) > 0 && r.IntN(2) == 0 { // the day's final @end
		last := slices.MaxFunc(stamps, func(a, b time.Time) int { return a.Compare(b) })
		at := last.Add(time.Duration(r.IntN(180)) * time.Minute)
		if g.today && at.After(g.now) {
			at = g.now
		}
		g.log.Entries = append(g.log.Entries, timestampedEntry(t, DayEndEntry, at.UTC()))
	}
	return g
}

// TestTimesheet_Oracle checks Timesheet against a direct computation
// over generated days. The oracle reads each minute of the day on its
// own: the entry that minute belongs to is the latest one logged at or
// before it (the last in the file, of several logged at the same
// time); a minute belongs to nothing before the day's first entry,
// after an @end, or, today, from now on. Summed by category, the
// minutes must give the sheet's per-category and worked time. The
// intervals themselves must be in order, not overlap, stay within the
// day (and, today, before now), and cover exactly the time from each
// stretch's first entry to its close: the @end after it, or the end of
// the day (24:00 on a past day, now today).
func TestTimesheet_Oracle(t *testing.T) {
	var zones []*time.Location
	for _, name := range []string{"UTC", "Europe/London", "America/New_York", "Asia/Kolkata", "Australia/Lord_Howe"} {
		zones = append(zones, mustZone(t, name))
	}
	categories := mustCategoriser(t, testCategories())
	r := rand.New(rand.NewPCG(20261118, 7))
	for i := range 3000 {
		g := generateDay(t, r, zones)
		s, err := g.log.Timesheet(categories, g.now)
		if err != nil {
			t.Fatalf("case %d: Timesheet: %v", i, err)
		}
		if msg := checkSheet(g, s, categories); msg != "" {
			var lines []string
			for _, e := range g.log.Entries {
				ts, _ := entryTimestamp(e)
				lines = append(lines, ts.In(g.start.Location()).Format("2006-01-02 15:04 ")+e.Entry())
			}
			var got []string
			for _, iv := range s.Intervals {
				got = append(got, describe(s, iv))
			}
			t.Fatalf("case %d (%s, now %s, today %v): %s\nentries:\n  %s\nintervals:\n  %s",
				i, g.start.Location(), g.now.Format("2006-01-02 15:04"), g.today, msg,
				strings.Join(lines, "\n  "), strings.Join(got, "\n  "))
		}
	}
}

// stampedEntry is an entry and its parsed ts:.
type stampedEntry struct {
	entry *Task
	at    time.Time
	index int // in the file
	cat   Category
	end   bool // IsDayEnd
}

// checkSheet returns what is wrong with s for g, or "".
func checkSheet(g genDay, s *Timesheet, c *Categoriser) string {
	var entries []stampedEntry
	for i, e := range g.log.Entries {
		at, _ := entryTimestamp(e)
		entries = append(entries, stampedEntry{e, at, i, c.Categorise(e), IsDayEnd(e)})
	}
	hi := g.end
	if g.today {
		hi = g.now
	}

	// The oracle, minute by minute.
	want := map[string]time.Duration{}
	var wantWorked time.Duration
	for m := g.start; m.Before(hi); m = m.Add(time.Minute) {
		var owner *stampedEntry
		for k := range entries {
			e := &entries[k]
			if !e.at.After(m) && (owner == nil || !e.at.Before(owner.at)) {
				owner = e
			}
		}
		if owner == nil || owner.end {
			continue
		}
		cat := owner.cat
		want[cat.Name] += time.Minute
		if cat.Work {
			wantWorked += time.Minute
		}
	}
	got := s.ByCategory()
	for name, d := range got {
		if d == 0 {
			delete(got, name) // the oracle has no zero-length intervals
		}
	}
	if !maps.Equal(got, want) {
		return fmt.Sprintf("ByCategory() = %v, oracle %v", got, want)
	}
	if s.Worked() != wantWorked {
		return fmt.Sprintf("Worked() = %v, oracle %v", s.Worked(), wantWorked)
	}

	// One interval per entry that isn't an @end, in order, within the
	// day, not overlapping.
	nonEnd := 0
	for _, e := range entries {
		if !IsDayEnd(e.entry) {
			nonEnd++
		}
	}
	if len(s.Intervals) != nonEnd {
		return fmt.Sprintf("%d intervals for %d entries that aren't @end", len(s.Intervals), nonEnd)
	}
	for k, iv := range s.Intervals {
		if iv.Start.Before(g.start) || iv.End.Before(iv.Start) || iv.End.After(hi) {
			return fmt.Sprintf("interval %d (%s) is outside [%s, %s] or ends before it starts", k, describe(s, iv), g.start, hi)
		}
		if k > 0 && iv.Start.Before(s.Intervals[k-1].End) {
			return fmt.Sprintf("interval %d starts before interval %d ends", k, k-1)
		}
		if iv.Category.Name != c.Categorise(iv.Entry).Name {
			return fmt.Sprintf("interval %d is in %s, its entry in %s", k, iv.Category.Name, c.Categorise(iv.Entry).Name)
		}
	}

	// Stretches: in time order, each run of entries that aren't @end
	// covers exactly [its first entry, its close], without gaps.
	slices.SortStableFunc(entries, func(a, b stampedEntry) int { return cmp.Compare(a.at.UnixNano(), b.at.UnixNano()) })
	clamp := func(at time.Time) time.Time {
		switch {
		case at.Before(g.start):
			return g.start
		case at.After(hi):
			return hi
		}
		return at
	}
	k := 0
	for i := 0; i < len(entries); {
		if IsDayEnd(entries[i].entry) {
			i++
			continue
		}
		j := i
		for j < len(entries) && !IsDayEnd(entries[j].entry) {
			j++
		}
		close := hi
		if j < len(entries) {
			close = clamp(entries[j].at)
		}
		first, last := s.Intervals[k], s.Intervals[k+j-i-1]
		if first.Entry != entries[i].entry || !first.Start.Equal(clamp(entries[i].at)) || !last.End.Equal(close) {
			return fmt.Sprintf("the stretch from entry %d covers [%s, %s], want [%s, %s]",
				entries[i].index, first.Start, last.End, clamp(entries[i].at), close)
		}
		for n := k + 1; n < k+j-i; n++ {
			if !s.Intervals[n].Start.Equal(s.Intervals[n-1].End) {
				return fmt.Sprintf("a gap inside a stretch, before interval %d", n)
			}
		}
		k += j - i
		i = j
	}

	// The day's status.
	var status DayStatus
	switch {
	case len(entries) == 0:
		status = DayEmpty
	case IsDayEnd(entries[len(entries)-1].entry):
		status = DayEnded
	case g.today:
		status = DayInProgress
	default:
		status = DayUnended
	}
	if s.Status != status {
		return fmt.Sprintf("Status = %v, want %v", s.Status, status)
	}
	if status == DayEnded && !s.EndedAt.Equal(entries[len(entries)-1].at) {
		return fmt.Sprintf("EndedAt = %v, want the final @end's %v", s.EndedAt, entries[len(entries)-1].at)
	}
	if (s.Current() != nil) != (status == DayInProgress) {
		return fmt.Sprintf("Current() = %v with status %v", s.Current(), status)
	}
	return ""
}
