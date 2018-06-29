package cmd

import (
	"strconv"

	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/hypotheticalco/tracker-client/utils"
	"github.com/spf13/cobra"
)

var do = &cobra.Command{
	Use:   "do [item number]",
	Short: "Complete a task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		utils.DieOnError("Supplied argument '"+args[0]+"' was not an integer: ", err)
		todo.CompleteTask(id)
	},
}

func init() {
	RootCmd.AddCommand(do)
}
