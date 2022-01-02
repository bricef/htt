package cmd

import (
	"strings"

	"github.com/bricef/htt/todo"
	"github.com/spf13/cobra"
)

var add = &cobra.Command{
	Use:     "add [task to add]",
	Aliases: []string{"a"},
	Short:   "Add an item to the current tasklist",
	Args:    cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		todo.AddTodo(strings.Join(args, " "))
		// todo.Show(todo.GetTodos())
	},
}

func init() {
	RootCmd.AddCommand(add)
}
