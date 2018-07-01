package cmd

import (
	"strconv"

	"github.com/hypotheticalco/tracker-client/timelogs"

	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/hypotheticalco/tracker-client/utils"
	"github.com/spf13/cobra"
)

var workOn = &cobra.Command{
	Use:   "workon [item number]",
	Short: "Log that work has began on numbered item.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		utils.DieOnError("Supplied argument '"+args[0]+"' was not an integer: ", err)
		t := todo.GetTodoID(id)
		timelogs.AddEntry(t.Entry)
	},
}

func init() {
	RootCmd.AddCommand(workOn)
}
