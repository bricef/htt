package cmd

import (
	"strings"

	"github.com/hypotheticalco/tracker-client/timelogs"
	"github.com/spf13/cobra"
)

var log = &cobra.Command{
	Use:   "log [search terms]",
	Short: "Log an entry to the time log.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		timelogs.AddEntry(strings.Join(args, " "))
	},
}

func init() {
	RootCmd.AddCommand(log)
}
