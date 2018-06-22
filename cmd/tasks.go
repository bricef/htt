package cmd

import (
	"strings"

	"github.com/hypotheticalco/tracker-client/models"
	"github.com/spf13/cobra"
)

func init() {

	RootCmd.AddCommand(&cobra.Command{
		Use:   "add",
		Short: "Add an item to the default task list",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			models.AddTodo(strings.Join(args, " "))
		},
	})

	RootCmd.AddCommand(&cobra.Command{
		Use:   "show [search terms]",
		Short: "Show the default tasklist.",
		Long: `Show the default tasklist. You can filter the task list using 
	search terms. Search terms will be matched fuzzily.`,
		Run: func(cmd *cobra.Command, args []string) {
			models.GetTodos(nil)
		},
	})

	// cmdTodo.AddCommand(cmdTodoAdd)
	// cmdTodo.AddCommand(cmdTodoComplete)
	// cmdTodo.AddCommand(cmdTodoRemove)
	// cmdTodo.AddCommand(cmdTodoRemove)
}
