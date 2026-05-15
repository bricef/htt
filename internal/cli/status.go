package cli

import (
	"fmt"

	"github.com/bricef/htt/internal/domain"
	"github.com/spf13/cobra"
)

var status = &cobra.Command{
	Use:     "status",
	Short:   "Show the status of the tasklist and time log.",
	Aliases: []string{"?"},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := printStatus(); err != nil {
			return err
		}

		current, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		names, err := domain.SwitchableContextNames(repo())
		if err != nil {
			return fmt.Errorf("list contexts: %w", err)
		}

		fmt.Printf("Available Contexts: ")
		for _, name := range names {
			c := &domain.Context{Name: name}
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
