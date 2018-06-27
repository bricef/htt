package cmd

import (
	"strconv"
	"strings"

	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/hypotheticalco/tracker-client/utils"
	"github.com/spf13/cobra"
)

var replace = &cobra.Command{
	Use:   "replace [item number] [new entry]",
	Short: "Replace an item with a new entry",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		utils.DieOnError("Supplied argument '"+args[0]+"' was not an integer: ", err)
		newEntry := strings.Join(args[1:], " ")
		todo.Replace(id, newEntry)
		todo.Show(todo.GetTodos())
	},
}

func init() {
	RootCmd.AddCommand(replace)
}
