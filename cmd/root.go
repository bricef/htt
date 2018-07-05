package cmd

import (
	"github.com/hypotheticalco/tracker-client/todo"
	"github.com/spf13/cobra"
)

// RootCmd is the top level command for ht
var RootCmd = &cobra.Command{
	Use:   "htt",
	Short: "Hypothetical Tasks & Time Tracker is a todo list manager and time tracker",
	Run: func(cmd *cobra.Command, args []string) {
		println(cmd.Short)
		println("For help and usage, use -h, --help or help")
		//cmd.Usage()
		todo.Show(todo.GetCurrentContext(), []string{})
	},
}
