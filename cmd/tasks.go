package cmd

import (
	"fmt"
	"strings"

	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/spf13/cobra"
)

func init() {

	RootCmd.AddCommand(&cobra.Command{
		Use:   "add",
		Short: "Add an item to the default task list",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			todo.AddTodo(strings.Join(args, " "))
		},
	})

	RootCmd.AddCommand(&cobra.Command{
		Use:   "show [search terms]",
		Short: "Show the default tasklist.",
		Long: `Show the default tasklist. You can filter the task list using 
	search terms. Search terms will be matched fuzzily.`,
		Run: func(cmd *cobra.Command, args []string) {
			todos := todo.GetTodos()
			filtered := todo.Filter(todos, []string{})
			for i, todo := range filtered {
				fmt.Printf("%3d %s\n", i+1, todo.Entry)
			}
			fmt.Printf("---\n")
			fmt.Printf("TODO %d of %d tasks shown", len(filtered), len(todos))
		},
	})

	// cmdTodo.AddCommand(cmdTodoAdd)
	// cmdTodo.AddCommand(cmdTodoComplete)
	// cmdTodo.AddCommand(cmdTodoRemove)
	// cmdTodo.AddCommand(cmdTodoRemove)
}
