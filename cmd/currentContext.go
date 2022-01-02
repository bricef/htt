package cmd

import (
	"fmt"

	"github.com/bricef/htt/todo"
	"github.com/spf13/cobra"
)

var currentContext = &cobra.Command{
	Use:   "currentContext",
	Short: "Outputs the current context",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(todo.GetCurrentContext())
	},
}

func init() {
	RootCmd.AddCommand(currentContext)
}
