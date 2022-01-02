package cmd

import (
	"github.com/bricef/htt/todo"
	"github.com/bricef/htt/utils"
	"github.com/spf13/cobra"
)

var editDone = &cobra.Command{
	Use:     "editDone",
	Short:   "Open the done file using $EDITOR",
	Args:    cobra.NoArgs,
	Aliases: []string{"ed"},
	Run: func(cmd *cobra.Command, args []string) {
		utils.EditFilePath(todo.DoneFilePath())
	},
}

func init() {
	RootCmd.AddCommand(editDone)
}
