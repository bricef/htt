package cmd

import (
	"github.com/bricef/htt/internal/timelogs"
	"github.com/bricef/htt/internal/todo"
	"github.com/spf13/cobra"
)

var status = &cobra.Command{
	Use:     "status",
	Short:   "Show the status of the tasklist and time log.",
	Aliases: []string{"?"},
	Run: func(cmd *cobra.Command, args []string) {
		timelogs.ShowStatus()
		todo.ShowStatus()
	},
}

func init() {
	RootCmd.AddCommand(status)
}
