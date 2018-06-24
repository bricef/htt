package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/hypotheticalco/tracker-client/utils"
	"github.com/spf13/cobra"
)

func init() {

	add := &cobra.Command{
		Use:   "add",
		Short: "Add an item to the default task list",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			todo.AddTodo(strings.Join(args, " "))
			todo.Show()
		},
	}
	RootCmd.AddCommand(add)

	show := &cobra.Command{
		Use:   "show [search terms]",
		Short: "Show the default tasklist.",
		Long: `Show the default tasklist. You can filter the task list using 
	search terms. Search terms will be matched fuzzily.`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 0 {
				utils.Failure("Filtering is not yet implemented. Showing without filter applied.")
			}
			todo.Show(todo.GetTodos())
		},
	}
	RootCmd.AddCommand(show)

	delete := &cobra.Command{
		Use:   "delete [item index]",
		Short: "Delete the item specified",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id, err := strconv.Atoi(args[0])
			utils.DieOnError("Supplied argument '"+args[0]+"' was not an integer: ", err)
			t := todo.GetTodoId(id)
			todo.Delete(t)
			fmt.Printf("Deleted task: " + t.Entry + "\n")
			fmt.Print("---")
			todo.Show(todo.GetTodos())
		},
	}
	RootCmd.AddCommand(delete)

	// cmdTodo.AddCommand(cmdTodoAdd)
	// cmdTodo.AddCommand(cmdTodoComplete)
	// cmdTodo.AddCommand(cmdTodoRemove)
	// cmdTodo.AddCommand(cmdTodoRemove)
}
