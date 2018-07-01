package cmd

import (
	"strings"

	"github.com/hypotheticalco/tracker-client/timelogs"
	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/spf13/cobra"
)

var addAndLog = &cobra.Command{
	Use:   "addLog [search terms]",
	Short: "Add an entry to the current tasklist and immediately start working on it.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		entry := strings.Join(args, " ")
		todo.AddTodo(entry)
		timelogs.AddEntry(entry)
	},
}

func init() {
	RootCmd.AddCommand(addAndLog)
}
