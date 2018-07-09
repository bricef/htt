package cmd

import (
	"github.com/hypotheticalco/tracker-client/timelogs"
	"github.com/spf13/cobra"
)

var workingOn = &cobra.Command{
	Use:     "workingOn",
	Short:   "Show the current active time log entry.",
	Args:    cobra.NoArgs,
	Aliases: []string{"wo"},
	Run: func(cmd *cobra.Command, args []string) {
		println(timelogs.CurrentActive())
	},
}

func init() {
	RootCmd.AddCommand(workingOn)
}
