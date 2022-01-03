package cmd

import (
	"github.com/bricef/htt/timelogs"
	"github.com/bricef/htt/todo"
	"github.com/bricef/htt/utils"
	"github.com/spf13/cobra"
)

var Begin = &cobra.Command{
	Use:     "workon [item number]",
	Short:   "Log that work has began on numbered item in the current context.",
	Args:    cobra.MinimumNArgs(1),
	Aliases: []string{"wo"},
	Run: func(cmd *cobra.Command, args []string) {
		ctx := todo.GetCurrentContext()
		t, err := ctx.GetTaskByStrId(args[0])
		utils.DieOnError("Could not find specified task.", err)
		timelogs.AddEntry(t)
	},
}

func init() {
	RootCmd.AddCommand(Begin)
}
