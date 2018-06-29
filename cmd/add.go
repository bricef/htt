package cmd

import (
	"strings"

	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/spf13/cobra"
)

var add = &cobra.Command{
	Use:     "add [task to add]",
	Aliases: []string{"a"},
	Short:   "Add an item to the current context list",
	Args:    cobra.MinimumNArgs(1),
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
