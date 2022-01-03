package cmd

import (
	"fmt"
	"strings"

	"github.com/bricef/htt/internal/timelogs"
	"github.com/bricef/htt/internal/todo"
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
	Run: func(cmd *cobra.Command, args []string) {
		task := todo.NewTask(strings.Join(args, " "))
		timelogs.AddEntry(task)
	},
}

var Show = &cobra.Command{
	Use:     "show",
	Short:   "Show today's time log.",
	Args:    cobra.NoArgs,
	Aliases: []string{"s"},
	Run: func(cmd *cobra.Command, args []string) {
		timelogs.Show()
	},
}

var Edit = &cobra.Command{
	Use:     "edit",
	Short:   "Open the current time log file using $EDITOR.",
	Args:    cobra.NoArgs,
	Aliases: []string{"e"},
	Run: func(cmd *cobra.Command, args []string) {
		utils.EditFilePath(timelogs.CurrentLogFilePath())
	},
}

var Status = &cobra.Command{
	Use:     "status",
	Short:   "Show the status of the time log.",
	Args:    cobra.NoArgs,
	Aliases: []string{"?"},
	Run: func(cmd *cobra.Command, args []string) {
		timelogs.ShowStatus()
	},
}

var Active = &cobra.Command{
	Use:   "active",
	Short: "Show active task, if any.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		current := timelogs.CurrentActive()

		fmt.Printf(
			"Working on: %s (%s)\n",
			current.RemoveAnnotation(timelogs.TimestampLabel).ColorString(),
			utils.HumanizeDuration(timelogs.CurrentDuration()))
	},
}

var Start = &cobra.Command{
	Use:   "start",
	Short: "Start the timelog for the day.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		timelogs.AddEntry(todo.NewTask(("@start")))
	},
}

var End = &cobra.Command{
	Use:   "end",
	Short: "End the timelog for the day.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		timelogs.AddEntry(todo.NewTask(("@end")))
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
