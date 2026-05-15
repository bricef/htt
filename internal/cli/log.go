package cli

import (
	"fmt"
	"strings"

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
// command shares: load today's Timelog, append the task, return the
// (now-annotated) task for display.
func appendToday(task *domain.Task) error {
	tl, err := timelogRepo().Today()
	if err != nil {
		return fmt.Errorf("load today's timelog: %w", err)
	}
	if err := tl.Append(task); err != nil {
		return fmt.Errorf("append log entry: %w", err)
	}
	return nil
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
		if err := appendToday(task); err != nil {
			return err
		}
		fmt.Printf("Logging entry: %v\n",
			task.RemoveAnnotation(domain.TimelogTimestampLabel).ColorString())
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
		utils.EditFilePath(path)
		return nil
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
		tl, err := timelogRepo().Today()
		if err != nil {
			return fmt.Errorf("load today's timelog: %w", err)
		}
		current := tl.Latest()
		if current == nil {
			fmt.Println("No active task.")
			return nil
		}
		dur, err := tl.Duration()
		if err != nil {
			return fmt.Errorf("read entry duration: %w", err)
		}
		fmt.Printf("Working on: %s (%s)\n",
			current.RemoveAnnotation(domain.TimelogTimestampLabel).ColorString(),
			utils.HumanizeDuration(dur))
		return nil
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
		return appendToday(task)
	},
}

var End = &cobra.Command{
	Use:   "end",
	Short: "End the timelog for the day.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		task, err := domain.NewTask("@end")
		if err != nil {
			return fmt.Errorf("build end entry: %w", err)
		}
		return appendToday(task)
	},
}

// printStatus is shared by `htt log status` and `htt status` (the outer
// rollup command). Mirrors the legacy timelogs.ShowStatus output:
// "Currently working on: <entry> (<duration>)" or "Not currently
// working on any task."
func printStatus() error {
	tl, err := timelogRepo().Today()
	if err != nil {
		return fmt.Errorf("load today's timelog: %w", err)
	}
	latest := tl.Latest()
	if latest == nil {
		fmt.Println("Not currently working on any task.")
		return nil
	}
	dur, err := tl.Duration()
	if err != nil {
		return fmt.Errorf("read entry duration: %w", err)
	}
	fmt.Printf("Currently working on: %v (%v) \n",
		latest.RemoveAnnotation(domain.TimelogTimestampLabel).ColorString(),
		utils.HumanizeDuration(dur))
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
