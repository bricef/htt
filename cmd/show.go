package cmd

import (
	"github.com/bricef/htt/todo"
	"github.com/bricef/htt/utils"
	"github.com/spf13/cobra"
)

var show = &cobra.Command{
	Use:     "show [search terms]",
	Aliases: []string{"s"},
	Short:   "Show the default tasklist.",
	Long: `Show the default tasklist. You can filter the task list using 
search terms. Search terms will be matched fuzzily.`,
	Run: func(cmd *cobra.Command, args []string) {
		// utils.ClearScreen()

		if len(args) > 0 {
			utils.Warning("Filtering is not yet implemented. Showing without filter applied.")
		}
		todo.Show(todo.GetCurrentContext(), args)
	},
}

func init() {
	RootCmd.AddCommand(show)
}
