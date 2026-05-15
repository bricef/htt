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
	Long: `Show tasks completed and time logged since a given date or
duration.

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
		now := time.Now()

		fmt.Printf("Activity since %s (%s)\n\n",
			since.Format("2006-01-02"),
			formatReportDuration(now.Sub(since)))

		if err := reportCompleted(since, now); err != nil {
			return err
		}
		fmt.Println()
		return reportTimeLogged(since, now)
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

// reportTimeLogged walks every calendar day in [since, until], loads
// its timelog, and prints the sum of Spans per day plus a grand
// total. Days without entries are skipped.
//
// The trailing-active entry is excluded by design (Spans only counts
// closed intervals). htt log status remains the way to see what's
// running right now.
func reportTimeLogged(since, until time.Time) error {
	fmt.Println("Time logged")

	type dayTotal struct {
		date     time.Time
		duration time.Duration
	}
	perDay := []dayTotal{}
	var total time.Duration

	day := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, since.Location())
	end := time.Date(until.Year(), until.Month(), until.Day(), 0, 0, 0, 0, until.Location())
	for !day.After(end) {
		tl, err := timelogRepo().Day(day)
		if err != nil {
			return fmt.Errorf("load timelog %s: %w", day.Format("2006-01-02"), err)
		}
		day = day.Add(24 * time.Hour)
		if tl.IsEmpty() {
			continue
		}
		spans, err := tl.Spans()
		if err != nil {
			return fmt.Errorf("spans for %s: %w", tl.Date.Format("2006-01-02"), err)
		}
		var d time.Duration
		for _, span := range spans {
			d += span.Duration
		}
		if d == 0 {
			continue
		}
		perDay = append(perDay, dayTotal{date: tl.Date, duration: d})
		total += d
	}

	if len(perDay) == 0 {
		fmt.Println("  (no time logged)")
		return nil
	}
	for _, dt := range perDay {
		fmt.Printf("  %s  %s\n", dt.date.Format("2006-01-02"), formatReportDuration(dt.duration))
	}
	fmt.Printf("  Total: %s (excluding currently-active entry)\n", formatReportDuration(total))
	return nil
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
