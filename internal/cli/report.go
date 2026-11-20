package cli

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/bricef/htt/internal/domain"
	"github.com/spf13/cobra"
)

var reportSince string

var Report = &cobra.Command{
	Use:   "report",
	Short: "Summarise activity in a time period.",
	Long: `Show what happened since a given date or duration: tasks
completed, tasks added, tasks deleted, and time logged per day.

Time logged gives each day's worked time and its time per category
(timelog.categories in the config), then the totals. A category's
line holds the entries its tags matched. Entries no category matched
are on a line of their own, uncategorised, which says the category
the time counts as (timelog.default_category, working unless set) and
so whether it is worked. An entry runs until the next one; @end ends
the day, and a day without one counts its last entry until midnight
and is flagged, with the command that ends it:
htt log end --at "YYYY-MM-DD HH:MM", the time you stopped. Today's
last entry counts until now, in progress.

--since takes a date (YYYY-MM-DD) or a duration counted back from now
in days, weeks or hours (7d, 2w, 24h).

Examples:
  htt report                 # defaults to --since 7d
  htt report --since 14d
  htt report --since 2w
  htt report --since 2026-05-09`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		since, err := parseSince(reportSince)
		if err != nil {
			return fmt.Errorf("parse --since: %w", err)
		}
		categories, err := timelogCategoriser()
		if err != nil {
			return err
		}
		now := time.Now()

		fmt.Printf("Activity since %s (%s)\n\n",
			since.Format("2006-01-02"),
			formatReportDuration(now.Sub(since)))

		if err := reportCompleted(since, now); err != nil {
			return err
		}
		fmt.Println()
		if err := reportAdded(since, now); err != nil {
			return err
		}
		fmt.Println()
		if err := reportDeleted(since, now); err != nil {
			return err
		}
		fmt.Println()
		return reportTimeLogged(since, now, categories)
	},
}

func init() {
	Report.Flags().StringVar(&reportSince, "since", "7d",
		"starting point: YYYY-MM-DD or Nd/Nw/Nh (e.g. 7d, 2w, 24h)")
	RootCmd.AddCommand(Report)
}

// parseSince accepts either an absolute date (YYYY-MM-DD) or a
// shorthand duration like 7d / 2w / 24h. Returns the absolute time
// `since` should rewind to.
//
// time.ParseDuration doesn't handle days or weeks (it caps at hours),
// so the shorthand parser is custom. Months and years are deferred
// because they're calendar-aware in ways that matter for reports.
func parseSince(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("expected YYYY-MM-DD or Nd/Nw/Nh, got %q", s)
	}
	unit := s[len(s)-1]
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n < 0 {
		return time.Time{}, fmt.Errorf("expected YYYY-MM-DD or Nd/Nw/Nh, got %q", s)
	}
	var d time.Duration
	switch unit {
	case 'd':
		d = time.Duration(n) * 24 * time.Hour
	case 'w':
		d = time.Duration(n) * 7 * 24 * time.Hour
	case 'h':
		d = time.Duration(n) * time.Hour
	default:
		return time.Time{}, fmt.Errorf("unknown unit %q in %q (use d, w, or h)", string(unit), s)
	}
	return time.Now().Add(-d), nil
}

// reportCompleted iterates the done context and prints tasks whose
// CompletedOn falls in [since, until], grouped by source context.
func reportCompleted(since, until time.Time) error {
	done, err := repo().Context(domain.DoneContextName)
	if err != nil {
		return fmt.Errorf("load done context: %w", err)
	}

	// Filter on calendar dates, not wall-clock times. done.txt only
	// stores dates (no time-of-day) and the parser surfaces them as
	// local midnight via time.ParseInLocation. Truncating since to
	// the start of its local day brings the comparison to date
	// level, so a task completed "today" still appears in reports
	// run shortly after the user's own local midnight.
	sinceDay := startOfDay(since)
	byCtx := map[string][]*domain.Task{}
	count := 0
	for _, task := range done.Tasks {
		if task.CompletedOn.IsZero() {
			continue
		}
		if task.CompletedOn.Before(sinceDay) || task.CompletedOn.After(until) {
			continue
		}
		src := task.Annotations["context"]
		if src == "" {
			src = "(unknown)"
		}
		byCtx[src] = append(byCtx[src], task)
		count++
	}

	fmt.Printf("Completed (%d)\n", count)
	if count == 0 {
		fmt.Println("  (none)")
		return nil
	}

	ctxNames := make([]string, 0, len(byCtx))
	for name := range byCtx {
		ctxNames = append(ctxNames, name)
	}
	sort.Strings(ctxNames)

	for _, name := range ctxNames {
		fmt.Printf("  %s:\n", name)
		entries := byCtx[name]
		// Stable order: by completion date ascending, then by entry text.
		sort.SliceStable(entries, func(i, j int) bool {
			if !entries[i].CompletedOn.Equal(entries[j].CompletedOn) {
				return entries[i].CompletedOn.Before(entries[j].CompletedOn)
			}
			return entries[i].Entry() < entries[j].Entry()
		})
		for _, t := range entries {
			display := t.Entry()
			if t.Priority != "" {
				display = fmt.Sprintf("(%s) %s", t.Priority, display)
			}
			fmt.Printf("    %-50s %s\n", display, t.CompletedOn.Format("2006-01-02"))
		}
	}
	return nil
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// reportAdded iterates every context but the archive (the open ones
// and done) and prints tasks whose CreatedOn falls in [since, until],
// grouped by source context.
//
// Open tasks count toward the context they currently live in; done
// tasks count toward the context recorded in their `context:`
// annotation (the source they were completed from). Deleted tasks are
// left out: they have their own section (reportDeleted), and walking
// the archive here listed them a second time, under a context named
// "archive". Tasks without a CreatedOn are skipped — that's mostly
// older entries created before Context.AddTask started stamping; they
// predate the feature and can't be backfilled without losing
// information.
func reportAdded(since, until time.Time) error {
	names, err := repo().ContextNames()
	if err != nil {
		return fmt.Errorf("list contexts: %w", err)
	}

	sinceDay := startOfDay(since)
	byCtx := map[string][]*domain.Task{}
	count := 0

	for _, name := range names {
		if name == domain.ArchiveContextName {
			continue
		}
		ctx, err := repo().Context(name)
		if err != nil {
			return fmt.Errorf("load context %s: %w", name, err)
		}
		for _, task := range ctx.Tasks {
			if task.CreatedOn.IsZero() {
				continue
			}
			if task.CreatedOn.Before(sinceDay) || task.CreatedOn.After(until) {
				continue
			}
			var src string
			if name == domain.DoneContextName {
				src = task.Annotations["context"]
				if src == "" {
					src = "(unknown)"
				}
			} else {
				src = name
			}
			byCtx[src] = append(byCtx[src], task)
			count++
		}
	}

	fmt.Printf("Added (%d)\n", count)
	if count == 0 {
		fmt.Println("  (none)")
		return nil
	}

	ctxNames := make([]string, 0, len(byCtx))
	for name := range byCtx {
		ctxNames = append(ctxNames, name)
	}
	sort.Strings(ctxNames)

	for _, name := range ctxNames {
		fmt.Printf("  %s:\n", name)
		entries := byCtx[name]
		sort.SliceStable(entries, func(i, j int) bool {
			if !entries[i].CreatedOn.Equal(entries[j].CreatedOn) {
				return entries[i].CreatedOn.Before(entries[j].CreatedOn)
			}
			return entries[i].Entry() < entries[j].Entry()
		})
		for _, t := range entries {
			display := t.Entry()
			if t.Priority != "" {
				display = fmt.Sprintf("(%s) %s", t.Priority, display)
			}
			fmt.Printf("    %-50s %s\n", display, t.CreatedOn.Format("2006-01-02"))
		}
	}
	return nil
}

// reportDeleted iterates the archive context and prints tasks whose
// deleted-on annotation falls in [since, until], grouped by their
// archived-from annotation.
//
// Entries missing or with malformed deleted-on are skipped rather
// than erroring the whole report — the archive accumulates over
// time and the cost of a single corrupt line shouldn't be a
// non-functional `htt report`.
func reportDeleted(since, until time.Time) error {
	archive, err := repo().Context(domain.ArchiveContextName)
	if err != nil {
		return fmt.Errorf("load archive context: %w", err)
	}

	sinceDay := startOfDay(since)
	byCtx := map[string][]archivedTask{}
	count := 0
	for _, task := range archive.Tasks {
		deletedOn, ok := domain.ParseLocalDate(task.Annotations["deleted-on"])
		if !ok {
			continue
		}
		if deletedOn.Before(sinceDay) || deletedOn.After(until) {
			continue
		}
		src := task.Annotations["archived-from"]
		if src == "" {
			src = "(unknown)"
		}
		byCtx[src] = append(byCtx[src], archivedTask{task: task, deletedOn: deletedOn})
		count++
	}

	fmt.Printf("Deleted (%d)\n", count)
	if count == 0 {
		fmt.Println("  (none)")
		return nil
	}

	ctxNames := make([]string, 0, len(byCtx))
	for name := range byCtx {
		ctxNames = append(ctxNames, name)
	}
	sort.Strings(ctxNames)

	for _, name := range ctxNames {
		fmt.Printf("  %s:\n", name)
		entries := byCtx[name]
		sort.SliceStable(entries, func(i, j int) bool {
			if !entries[i].deletedOn.Equal(entries[j].deletedOn) {
				return entries[i].deletedOn.Before(entries[j].deletedOn)
			}
			return entries[i].task.Entry() < entries[j].task.Entry()
		})
		for _, e := range entries {
			display := e.task.Entry()
			if e.task.Priority != "" {
				display = fmt.Sprintf("(%s) %s", e.task.Priority, display)
			}
			fmt.Printf("    %-50s %s\n", display, e.deletedOn.Format("2006-01-02"))
		}
	}
	return nil
}

// archivedTask pairs a task with its parsed deleted-on date so the
// sort comparator can avoid re-parsing the annotation per call.
type archivedTask struct {
	task      *domain.Task
	deletedOn time.Time
}

// reportTimeLogged reads every calendar day from since's to until's
// (domain.ReadDays: each day's time log as a timesheet at until, now),
// and prints each day's worked time and its time per category, then the
// totals over the days (domain.Sum); see printDays.
func reportTimeLogged(since, until time.Time, categories *domain.Categoriser) error {
	fmt.Println("Time logged")
	// Days by their dates in the local zone, until's, each at noon, as a
	// domain.Period holds them: a day the clocks change isn't 24 hours
	// long, and where they go forward at midnight, that midnight
	// doesn't exist.
	loc := until.Location()
	days, err := domain.ReadDays(timelogRepo(), categories, domain.Period{
		First: time.Date(since.Year(), since.Month(), since.Day(), 12, 0, 0, 0, loc),
		Last:  time.Date(until.Year(), until.Month(), until.Day(), 12, 0, 0, 0, loc),
	}.Dates(), until)
	if err != nil {
		return err
	}
	printDays(days, domain.Sum(days), categories)
	return nil
}

// printDays prints each day's worked time and its time per category,
// then total, the days' totals, under the section's heading:
//
//	Time logged
//	  2026-11-02   7h 0m worked
//	    break           1h 0m
//	    htt             4h 0m
//	    uncategorised   3h 0m  (counted as working)
//	  2026-11-03  15h 0m worked (no @end: counted to 24:00)
//	              end the day with htt log end --at "2026-11-03 HH:MM"
//	    uncategorised  15h 0m  (counted as working)
//	  Total: 22h 0m worked
//	    break           1h 0m
//	    htt             4h 0m
//	    uncategorised  18h 0m  (counted as working)
//
// A day without an @end counts its last entry to midnight, and says
// so, with the command that ends it on a line of its own (the day's
// line would be too long for a terminal with it); today's last entry
// counts until now, in progress. A
// category's line is the time its tags matched; the time of entries no
// category matched is on the uncategorised line, which names the
// default category it counts as (whose work setting decides whether it
// is worked). Days with no time logged are left out (today in progress
// isn't), and so are lines with none; with no day left, it says so.
// Categories are listed in the config's order, then uncategorised.
func printDays(days []*domain.Timesheet, total domain.Totals, categories *domain.Categoriser) {
	var shown []*domain.Timesheet
	for _, d := range days {
		if d.Totals().Logged() > 0 || d.Status == domain.DayInProgress {
			shown = append(shown, d)
		}
	}
	if len(shown) == 0 {
		fmt.Println("  (no time logged)")
		return
	}

	// Columns: category names, and durations lined up on the right.
	nameWidth, durWidth, dayWidth := 0, 0, 0
	for _, cat := range categories.Categories() {
		if d := total.ByCategory[cat.Name]; d > 0 {
			nameWidth = max(nameWidth, len(cat.Name))
			durWidth = max(durWidth, len(formatReportDuration(d)))
		}
	}
	if total.Uncategorised > 0 {
		nameWidth = max(nameWidth, len(uncategorisedLabel))
		durWidth = max(durWidth, len(formatReportDuration(total.Uncategorised)))
	}
	for _, d := range shown {
		t := d.Totals()
		dayWidth = max(dayWidth, len(formatReportDuration(t.Worked)))
		for _, dur := range t.ByCategory {
			durWidth = max(durWidth, len(formatReportDuration(dur)))
		}
	}
	printCategories := func(byName map[string]time.Duration, uncat time.Duration) {
		for _, cat := range categories.Categories() {
			if d := byName[cat.Name]; d > 0 {
				fmt.Printf("    %-*s  %*s\n", nameWidth, cat.Name, durWidth, formatReportDuration(d))
			}
		}
		if uncat > 0 {
			fmt.Printf("    %-*s  %*s  (counted as %s)\n", nameWidth, uncategorisedLabel, durWidth,
				formatReportDuration(uncat), categories.Default().Name)
		}
	}

	for _, d := range shown {
		t := d.Totals()
		date := d.Date.Format("2006-01-02")
		note := ""
		switch d.Status {
		case domain.DayUnended:
			note = " (no @end: counted to 24:00)"
		case domain.DayInProgress:
			note = " (in progress)"
		}
		fmt.Printf("  %s  %*s worked%s\n", date, dayWidth, formatReportDuration(t.Worked), note)
		if d.Status == domain.DayUnended {
			// Under the day's time, how to end it.
			fmt.Printf("  %*s  end the day with htt log end --at %q\n", len(date), "", date+" HH:MM")
		}
		printCategories(t.ByCategory, t.Uncategorised)
	}
	fmt.Printf("  Total: %s worked\n", formatReportDuration(total.Worked))
	printCategories(total.ByCategory, total.Uncategorised)
}

// formatReportDuration prints a duration as "Xh Ym" or "Ym" — easier
// on the eye in a column than utils.HumanizeDuration's "4h30m" form,
// and handles durations >= 24h without falling back to "Xh Ym Ws".
func formatReportDuration(d time.Duration) string {
	if d < time.Minute {
		return "0m"
	}
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}
