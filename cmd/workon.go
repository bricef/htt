package cmd

import (
	"strconv"

	"github.com/bricef/htt/timelogs"

	"github.com/bricef/htt/todo"
	"github.com/bricef/htt/utils"
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
		timelogs.AddEntry(t.ToString())
	},
}

func init() {
	RootCmd.AddCommand(workOn)
}
