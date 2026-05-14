package cmd

import (
	"fmt"

	"github.com/bricef/htt/internal/timelogs"
	"github.com/bricef/htt/internal/todo"
	"github.com/spf13/cobra"
)

var status = &cobra.Command{
	Use:     "status",
	Short:   "Show the status of the tasklist and time log.",
	Aliases: []string{"?"},
	RunE: func(cmd *cobra.Command, args []string) error {
		timelogs.ShowStatus()

		current, err := uc().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		names, err := uc().ListContextNames()
		if err != nil {
			return fmt.Errorf("list contexts: %w", err)
		}

		fmt.Printf("Available Contexts: ")
		for _, name := range names {
			c := &todo.Context{Name: name}
			fmt.Printf("%s ", c.ConsoleString())
		}
		fmt.Println()
		fmt.Printf("Current Context: %s\n", current.ConsoleString())
		current.Show()
		return nil
	},
}

func init() {
	RootCmd.AddCommand(status)
}
