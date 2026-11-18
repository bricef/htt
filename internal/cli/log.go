package cli

import (
	"fmt"
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

var Status = &cobra.Command{
	Use:     "status",
	Short:   "Show the status of the time log.",
	Args:    cobra.NoArgs,
	Aliases: []string{"?"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return printStatus()
	},
}

var Active = &cobra.Command{
	Use:   "active",
	Short: "Show active task, if any.",
	Args:  cobra.NoArgs,
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
// rollup command): "Currently working on: <entry> (<duration>)", "Not
// working (day ended at HH:MM)" after an @end, or "Not currently
// working on any task." on a day with nothing logged.
func printStatus() error {
	return printToday("Not currently working on any task.", "Currently working on:")
}

// printToday prints where today's time log stands: the entry running
// now and how long it has run, after the working prefix; that the day
// ended, and when, after an @end; or the empty line on a day with
// nothing logged. `log status` and `log active` word those two lines
// differently. Neither needs the categories, so a bad timelog config
// doesn't stop them.
func printToday(empty, working string) error {
	tl, err := timelogRepo().Today()
	if err != nil {
		return fmt.Errorf("load today's timelog: %w", err)
	}
	now := time.Now()
	sheet, err := tl.Timesheet(nil, now)
	if err != nil {
		return fmt.Errorf("read today's timelog: %w", err)
	}
	switch sheet.Status {
	case domain.DayEnded:
		fmt.Printf("Not working (day ended at %s)\n", sheet.EndedAt.In(now.Location()).Format("15:04"))
	case domain.DayInProgress:
		current := sheet.Current()
		fmt.Printf("%s %s (%s)\n", working,
			current.Entry.RemoveAnnotation(domain.TimelogTimestampLabel).ColorString(),
			utils.HumanizeDuration(current.Duration()))
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
