package cmd

import (
	"fmt"

	"github.com/bricef/htt/timelogs"
	"github.com/spf13/cobra"
)

var workingOn = &cobra.Command{
	Use:     "workingon",
	Short:   "Show the current active time log entry.",
	Args:    cobra.NoArgs,
	Aliases: []string{"wo"},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf(timelogs.CurrentActive())
	},
}

func init() {
	RootCmd.AddCommand(workingOn)
}
