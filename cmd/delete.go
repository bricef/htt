package cmd

import (
	"fmt"
	"strconv"

	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/hypotheticalco/tracker-client/utils"
	"github.com/spf13/cobra"
)

var delete = &cobra.Command{
	Use:   "delete [item index]",
	Short: "Delete the item specified",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		utils.DieOnError("Supplied argument '"+args[0]+"' was not an integer: ", err)
		t := todo.GetTodoID(id)
		todo.Delete(t)
		fmt.Printf("Deleted task: " + t.Entry + "\n")
		fmt.Print("---")
		// todo.Show(todo.GetTodos())
	},
}

func init() {
	RootCmd.AddCommand(delete)
}
