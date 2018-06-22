package cmd

import (
	"github.com/spf13/cobra"
)

// RootCmd is the top level command for ht
var RootCmd = &cobra.Command{
	Use:   "ht",
	Short: "Hypothetical tracker is a todo list manager and time tracker",
	Run: func(cmd *cobra.Command, args []string) {
		println(cmd.Short)
		println()
		cmd.Usage()
	},
}
