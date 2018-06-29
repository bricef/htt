package cmd

import (
	"strings"

	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/spf13/cobra"
)

var addTo = &cobra.Command{
	Use:   "addTo [task to add]",
	Short: "Add an item to a specified context list",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		context := args[0]
		entry := strings.Join(args[1:], " ")
		todo.AddToContext(context, entry)
		// todo.Show(todo.GetTodos())
	},
}

func init() {
	RootCmd.AddCommand(addTo)
}
