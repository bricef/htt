package cli

import (
	"fmt"
	"strings"

	"github.com/bricef/htt/internal/domain"
	"github.com/bricef/htt/internal/timelogs"
	"github.com/bricef/htt/internal/utils"
	"github.com/spf13/cobra"
)

var LogCommand = &cobra.Command{
	Use:     "log",
	Short:   "Manage the activity log.",
	Args:    cobra.NoArgs,
	Aliases: []string{"l"},
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
		timelogs.AddEntry(task)
		return nil
	},
}

var Show = &cobra.Command{
	Use:     "show",
	Short:   "Show today's time log.",
	Args:    cobra.NoArgs,
	Aliases: []string{"s"},
	RunE: func(cmd *cobra.Command, args []string) error {
		timelogs.Show()
		return nil
	},
}

var Edit = &cobra.Command{
	Use:     "edit",
	Short:   "Open the current time log file using $EDITOR.",
	Args:    cobra.NoArgs,
	Aliases: []string{"e"},
	RunE: func(cmd *cobra.Command, args []string) error {
		utils.EditFilePath(timelogs.CurrentLogFilePath())
		return nil
	},
}

var Status = &cobra.Command{
	Use:     "status",
	Short:   "Show the status of the time log.",
	Args:    cobra.NoArgs,
	Aliases: []string{"?"},
	RunE: func(cmd *cobra.Command, args []string) error {
		timelogs.ShowStatus()
		return nil
	},
}

var Active = &cobra.Command{
	Use:   "active",
	Short: "Show active task, if any.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// bug_004: timelogs.CurrentActive returns nil for an absent or
		// empty log file (fresh install, day before first entry). The
		// old code dereferenced unconditionally and panicked.
		current := timelogs.CurrentActive()
		if current == nil {
			fmt.Println("No active task.")
			return nil
		}
		fmt.Printf(
			"Working on: %s (%s)\n",
			current.RemoveAnnotation(timelogs.TimestampLabel).ColorString(),
			utils.HumanizeDuration(timelogs.CurrentDuration()))
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
		timelogs.AddEntry(task)
		return nil
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
		timelogs.AddEntry(task)
		return nil
	},
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
