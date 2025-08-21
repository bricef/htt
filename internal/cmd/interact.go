package cmd

import (
	"fmt"
	"os"

	"github.com/bricef/htt/internal/interactive"
	"github.com/bricef/htt/internal/todo"
	"github.com/spf13/cobra"

	tea "github.com/charmbracelet/bubbletea"
)

var Interactive = &cobra.Command{
	Use:     "interactive",
	Short:   "Enter interactive mode",
	Args:    cobra.NoArgs,
	Aliases: []string{"i"},
	Run: func(cmd *cobra.Command, args []string) {
		ctx := todo.GetCurrentContext()
		p := tea.NewProgram(
			interactive.Model(ctx),
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		)

		{ // Log to file
			f, err := tea.LogToFile("debug.log", "debug")
			if err != nil {
				fmt.Printf("Alas, there's been an error: %v", err)
				os.Exit(1)
			}
			defer f.Close()
		}
		if _, err := p.Run(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(Interactive)
}
