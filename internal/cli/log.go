package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/utils"
	"github.com/spf13/cobra"
)

var LogCommand = &cobra.Command{
	Use:     "log",
	Short:   "Manage the activity log.",
	Args:    cobra.NoArgs,
	Aliases: []string{"l"},
}

// appendToday is the small helper every "write a timelog entry"
// command shares (log add, start and end, and workon): load today's
// Timelog, append the task, and confirm the entry, without the ts:
// annotation Append gives it. It returns the Timelog, which now holds
// the task.
func appendToday(task *domain.Task) (*domain.Timelog, error) {
	tl, err := timelogRepo().Today()
	if err != nil {
		return nil, fmt.Errorf("load today's timelog: %w", err)
	}
	if err := tl.Append(task); err != nil {
		return nil, fmt.Errorf("append log entry: %w", err)
	}
	fmt.Printf("Logging entry: %v\n",
		task.RemoveAnnotation(domain.TimelogTimestampLabel).ColorString())
	return tl, nil
}

var Add = &cobra.Command{
	Use:     "add [entry]",
	Short:   "Log an entry to the time log.",
	Args:    cobra.MinimumNArgs(1),
	Aliases: []string{"+"},
	RunE: func(cmd *cobra.Command, args []string) error {
		task, err := domain.NewTask(strings.Join(args, " "))
		if err != nil {
			return fmt.Errorf("parse log entry: %w", err)
		}
		tl, err := appendToday(task)
		if err != nil {
			return err
		}
		warnTypedEntry(tl, task)
		return nil
	},
}

var Show = &cobra.Command{
	Use:     "show",
	Short:   "Show today's time log.",
	Args:    cobra.NoArgs,
	Aliases: []string{"s"},
	RunE: func(cmd *cobra.Command, args []string) error {
		tl, err := timelogRepo().Today()
		if err != nil {
			return fmt.Errorf("load today's timelog: %w", err)
		}
		if tl.IsEmpty() {
			fmt.Println("Today's timelog does not yet exist. Add an entry.")
			return nil
		}
		for _, entry := range tl.Entries {
			fmt.Println(entry.Raw)
		}
		return nil
	},
}

var Edit = &cobra.Command{
	Use:     "edit",
	Short:   "Open the current time log file using $EDITOR.",
	Args:    cobra.NoArgs,
	Aliases: []string{"e"},
	RunE: func(cmd *cobra.Command, args []string) error {
		path := timelogRepo().CurrentLogPath()
		if path == "" {
			return fmt.Errorf("active timelog repository does not expose a file path")
		}
		return utils.EditFilePath(path)
	},
}

// todayHelp ends the help of log status and log active, which differ
// only in their first lines.
const todayHelp = `The category is the first in timelog.categories with one of the
entry's tags, or uncategorised when none has one; uncategorised time
counts as work or not as timelog.default_category says (working
unless set). After htt log end it says "Not working (day ended at
HH:MM)".

A timelog config htt can't use doesn't stop it: it warns on stderr
and shows the entry without its category.`

var Status = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the time log.",
	Long: `Show the entry you're on today, its category and how long it has
run: "Currently working on: <entry> (<category>, <time>)", or "Not
working: <entry> (<category>, <time>)" when the category doesn't
count as work, such as a break.

` + todayHelp,
	Args:    cobra.NoArgs,
	Aliases: []string{"?"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return printStatus()
	},
}

var Active = &cobra.Command{
	Use:   "active",
	Short: "Show active task, if any.",
	Long: `Show the entry you're on today, its category and how long it has
run: "Working on: <entry> (<category>, <time>)", or "Not working:
<entry> (<category>, <time>)" when the category doesn't count as
work, such as a break.

` + todayHelp,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return printToday("No active task.", "Working on:")
	},
}

var Start = &cobra.Command{
	Use:   "start",
	Short: "Start the timelog for the day.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		task, err := domain.NewTask("@start")
		if err != nil {
			return fmt.Errorf("build start entry: %w", err)
		}
		_, err = appendToday(task)
		return err
	},
}

var End = &cobra.Command{
	Use:   "end",
	Short: "End the day in the time log.",
	Long: `Log an @end entry, which ends the day: the entry before it stops
there. An entry logged after it starts again, and the time between
them counts as nothing.

A day without an @end counts its last entry until midnight, and
htt report flags it.

For a break, log an entry rather than ending the day, such as
htt log add @lunch, and give it a category that doesn't count as work
(see timelog.categories in GETTING-STARTED.md).`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		task, err := domain.NewTask("@end")
		if err != nil {
			return fmt.Errorf("build end entry: %w", err)
		}
		_, err = appendToday(task)
		return err
	},
}

// printStatus is shared by `htt log status` and `htt status` (the outer
// rollup command): "Currently working on: <entry> (<category>,
// <duration>)", "Not working: <entry> (<category>, <duration>)" on an
// entry whose category isn't work, "Not working (day ended at HH:MM)"
// after an @end, or "Not currently working on any task." on a day with
// nothing logged.
func printStatus() error {
	return printToday("Not currently working on any task.", "Currently working on:")
}

// notWorking is printToday's prefix for an entry whose category doesn't
// count as work, such as a break, from log status and log active alike.
const notWorking = "Not working:"

// printToday prints where today's time log stands: the entry running
// now, its category (uncategorisedLabel if none matched) and how long
// it has run, after the working prefix, or after notWorking when its
// category doesn't count as work; that the day ended, and when, after
// an @end; or the empty line on a day with nothing logged. `log status`
// and `log active` word the working and empty lines differently.
//
// A timelog config htt can't use doesn't stop it, as it stops the
// report: it warns on stderr and prints the entry without its category
// (every entry working), as status did before it read the categories.
// It warns only when it shows an entry, the only line a category
// changes.
func printToday(empty, working string) error {
	tl, err := timelogRepo().Today()
	if err != nil {
		return fmt.Errorf("load today's timelog: %w", err)
	}
	categories, configErr := timelogCategoriser() // nil on an error: no categories
	now := time.Now()
	sheet, err := tl.Timesheet(categories, now)
	if err != nil {
		return fmt.Errorf("read today's timelog: %w", err)
	}
	switch sheet.Status {
	case domain.DayEnded:
		fmt.Printf("Not working (day ended at %s)\n", sheet.EndedAt.In(now.Location()).Format("15:04"))
	case domain.DayInProgress:
		current := sheet.Current()
		entry := current.Entry.RemoveAnnotation(domain.TimelogTimestampLabel).ColorString()
		running := utils.HumanizeDuration(current.Duration())
		if configErr != nil {
			fmt.Fprintf(os.Stderr, "warning: %v; showing entries without their categories\n", configErr)
			fmt.Printf("%s %s (%s)\n", working, entry, running)
			return nil
		}
		prefix, category := working, current.Category.Name
		if !current.Category.Work {
			prefix = notWorking
		}
		if current.Uncategorised {
			category = uncategorisedLabel
		}
		fmt.Printf("%s %s (%s, %s)\n", prefix, entry, category, running)
	default: // nothing logged today
		fmt.Println(empty)
	}
	return nil
}

func init() {
	LogCommand.AddCommand(Add)
	LogCommand.AddCommand(Show)
	LogCommand.AddCommand(Edit)
	LogCommand.AddCommand(Status)
	LogCommand.AddCommand(Active)
	LogCommand.AddCommand(Start)
	LogCommand.AddCommand(End)

	RootCmd.AddCommand(LogCommand)
}
