package cmd

import (
	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/spf13/cobra"
)

var currentContext = &cobra.Command{
	Use:   "currentContext",
	Short: "Outputs the current context",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		println(todo.GetCurrentContext())
	},
}

func init() {
	RootCmd.AddCommand(currentContext)
}
