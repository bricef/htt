package cmd

import (
	"github.com/hypotheticalco/tracker-client/timelogs"
	"github.com/hypotheticalco/tracker-client/utils"
	"github.com/spf13/cobra"
)

var editLog = &cobra.Command{
	Use:     "editLog",
	Short:   "Open the current time log file using $EDITOR",
	Args:    cobra.NoArgs,
	Aliases: []string{"el"},
	Run: func(cmd *cobra.Command, args []string) {
		utils.EditFilePath(timelogs.CurrentLogFilePath())
	},
}

func init() {
	RootCmd.AddCommand(editLog)
}
