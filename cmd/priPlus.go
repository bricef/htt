package cmd

import (
	"strconv"

	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/hypotheticalco/tracker-client/utils"
	"github.com/spf13/cobra"
)

var priPlus = &cobra.Command{
	Use:   "pri+ [item number]",
	Short: "increase the priority for the selected task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		id, err := strconv.Atoi(args[0])
		utils.DieOnError("Supplied argument '"+args[0]+"' was not an integer: ", err)
		t := todo.GetTodoID(id)

		todo.Replace(id, t.IncreasePriority())

	},
}

func init() {
	RootCmd.AddCommand(priPlus)
}
