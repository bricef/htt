package cmd

import (
	"github.com/bricef/htt/timelogs"
	"github.com/spf13/cobra"
)

var showLog = &cobra.Command{
	Use:     "showLog",
	Short:   "Show the day's time log.",
	Aliases: []string{"sl"},
	Run: func(cmd *cobra.Command, args []string) {
		timelogs.Show()
	},
}

func init() {
	RootCmd.AddCommand(showLog)
}
