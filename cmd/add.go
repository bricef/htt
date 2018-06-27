package cmd

import (
	"strings"

	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/spf13/cobra"
)

var add = &cobra.Command{
	Use:   "add",
	Short: "Add an item to the default task list",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		todo.AddTodo(strings.Join(args, " "))
		todo.Show(todo.GetTodos())
	},
}

func init() {
	RootCmd.AddCommand(add)
	// deduplicate
	// up-pri
	// down-pri
	// do
	// report
}
