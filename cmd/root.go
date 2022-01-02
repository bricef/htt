package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// RootCmd is the top level command for ht
var RootCmd = &cobra.Command{
	Use:   "htt",
	Short: "htt is a command line todo list manager and time tracker",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Short)
		fmt.Println()
		cmd.Usage()

	},
}
