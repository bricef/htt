package cli

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bricef/htt/internal/domain"
	"github.com/spf13/cobra"
)

var (
	showSortKey string
	showFilters []string
)

var show = &cobra.Command{
	Use:     "show",
	Aliases: []string{"s", "ls", "l"},
	Short:   "Show the current tasklist.",
	Long: `Show the current tasklist.

With no flags, tasks are listed in file order. --sort and --filter
narrow and reorder the list instead:

  --sort KEY      priority, created or due. Prefix with - to reverse
                  (-created lists the newest first). Tasks with no
                  created date or no due date stay at the bottom in
                  either direction.
  --filter EXPR   overdue, due-soon (due in the next 7 days), no-due,
                  has-tag:NAME (matches @NAME, +NAME or #NAME; write
                  has-tag:+NAME for one kind only) or priority:A|B|C.
                  Repeat the flag to combine filters: a task must
                  match all of them.

The number beside each task is its position in the full list, not in
the view, so "htt todo do 3" or "htt todo delete 3" still targets the
task you see however the view is sorted.

Examples:
  htt todo show --sort -created
  htt todo show --filter overdue --sort due
  htt todo show --filter priority:A --filter due-soon`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		if showSortKey == "" && len(showFilters) == 0 {
			ctx.Show()
			return nil
		}
		view, err := buildView(ctx, showSortKey, showFilters, time.Now())
		if err != nil {
			return err
		}
		printView(ctx, view)
		return nil
	},
}

func init() {
	show.Flags().StringVar(&showSortKey, "sort", "",
		"Order by priority, created or due; prefix with - to reverse")
	show.Flags().StringSliceVar(&showFilters, "filter", nil,
		"Keep only tasks matching: overdue, due-soon, no-due, has-tag:NAME, priority:A|B|C (repeatable)")
}

// buildView returns the tasks in ctx that match every filter
// expression, ordered by sortKey (file order when sortKey is empty).
// ctx.Tasks is left untouched. The tasks keep their Line, which is
// what printView shows as each task's index.
func buildView(ctx *domain.Context, sortKey string, filters []string, now time.Time) ([]*domain.Task, error) {
	var compare func(a, b *domain.Task) int
	if sortKey != "" {
		c, err := parseSortKey(sortKey)
		if err != nil {
			return nil, err
		}
		compare = c
	}

	predicates := make([]func(*domain.Task) bool, 0, len(filters))
	for _, expr := range filters {
		p, err := parseFilter(expr, now)
		if err != nil {
			return nil, err
		}
		predicates = append(predicates, p)
	}

	view := ctx.Search(func(t *domain.Task) bool {
		for _, p := range predicates {
			if !p(t) {
				return false
			}
		}
		return true
	})
	if compare != nil {
		slices.SortStableFunc(view, compare)
	}
	return view, nil
}

// printView renders a view in the same row format as Context.Show,
// with a footer saying how much of the list the view covers.
func printView(ctx *domain.Context, view []*domain.Task) {
	if len(ctx.Tasks) == 0 {
		ctx.Show()
		return
	}
	if len(view) == 0 {
		fmt.Printf("(%s): No tasks matched query.\n", ctx.ConsoleString())
		return
	}
	fmt.Println()
	for _, t := range view {
		fmt.Printf("%3d %s\n", t.Line, t.ConsoleString())
	}
	fmt.Println()
	fmt.Printf("(%s): %d of %d tasks shown.\n", ctx.ConsoleString(), len(view), len(ctx.Tasks))
}

// sortAxis is one --sort key. has reports whether a task carries a
// value on the axis; compare orders two tasks that both do.
type sortAxis struct {
	has     func(*domain.Task) bool
	compare func(a, b *domain.Task) int
}

var sortAxes = map[string]sortAxis{
	// No priority ranks below C, as in Context.Sort. Every task has a
	// value on this axis, so -priority puts unprioritised tasks first.
	"priority": {
		has: func(*domain.Task) bool { return true },
		compare: func(a, b *domain.Task) int {
			return strings.Compare(priorityRank(a.Priority), priorityRank(b.Priority))
		},
	},
	"created": {
		has: func(t *domain.Task) bool { return !t.CreatedOn.IsZero() },
		compare: func(a, b *domain.Task) int {
			return a.CreatedOn.Compare(b.CreatedOn)
		},
	},
	"due": {
		has: func(t *domain.Task) bool {
			_, ok := dueDate(t)
			return ok
		},
		compare: func(a, b *domain.Task) int {
			ad, _ := dueDate(a)
			bd, _ := dueDate(b)
			return ad.Compare(bd)
		},
	},
}

// parseSortKey resolves a --sort value into a comparator for
// slices.SortStableFunc. A leading "-" reverses the order of tasks
// that have a value; tasks without one sort last in both directions,
// so "-due" lists the furthest deadline first and still leaves
// undated tasks at the bottom.
func parseSortKey(key string) (func(a, b *domain.Task) int, error) {
	name, descending := strings.CutPrefix(key, "-")
	axis, ok := sortAxes[name]
	if !ok {
		return nil, fmt.Errorf("invalid --sort %q (want priority, created or due, optionally prefixed with -)", key)
	}
	return func(a, b *domain.Task) int {
		aHas, bHas := axis.has(a), axis.has(b)
		switch {
		case aHas && bHas:
			if descending {
				return axis.compare(b, a)
			}
			return axis.compare(a, b)
		case aHas:
			return -1
		case bHas:
			return 1
		default:
			return 0
		}
	}, nil
}

// parseFilter resolves a --filter expression into a predicate. Due
// dates are compared as calendar days against now's local date. A
// due: value that isn't a YYYY-MM-DD date counts as no due date, the
// same as for --sort due, so such tasks surface under no-due.
func parseFilter(expr string, now time.Time) (func(*domain.Task) bool, error) {
	today := startOfDay(now)
	switch expr {
	case "overdue":
		return func(t *domain.Task) bool {
			due, ok := dueDate(t)
			return ok && due.Before(today)
		}, nil
	case "due-soon":
		horizon := today.AddDate(0, 0, 7)
		return func(t *domain.Task) bool {
			due, ok := dueDate(t)
			return ok && !due.Before(today) && due.Before(horizon)
		}, nil
	case "no-due":
		return func(t *domain.Task) bool {
			_, ok := dueDate(t)
			return !ok
		}, nil
	}
	if name, ok := strings.CutPrefix(expr, "has-tag:"); ok && name != "" {
		return func(t *domain.Task) bool { return hasTag(t, name) }, nil
	}
	if letter, ok := strings.CutPrefix(expr, "priority:"); ok {
		p := strings.ToUpper(letter)
		if p == "A" || p == "B" || p == "C" {
			return func(t *domain.Task) bool { return t.Priority == p }, nil
		}
	}
	return nil, fmt.Errorf("invalid --filter %q (want overdue, due-soon, no-due, has-tag:NAME or priority:A|B|C)", expr)
}

// dueDate returns the task's due: annotation as a local date, or
// false when it is missing or malformed.
func dueDate(t *domain.Task) (time.Time, bool) {
	return parseAnnotationDate(t.Annotations["due"])
}

// hasTag reports whether the task carries the tag name. Tag values
// keep their sigil ("@home"), so a bare name matches any sigil and a
// sigiled name ("+home") matches only that kind.
func hasTag(t *domain.Task, name string) bool {
	for sigil, values := range t.Tags {
		for _, v := range values {
			if v == name || v == sigil+name {
				return true
			}
		}
	}
	return false
}

func priorityRank(p string) string {
	if p == "" {
		return "Z"
	}
	return p
}
