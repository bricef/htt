package cmd

import (
	"fmt"

	"github.com/bricef/htt/internal/interactive"
	"github.com/spf13/cobra"

	tea "github.com/charmbracelet/bubbletea"
)

var debug = false

var Interactive = &cobra.Command{
	Use:     "interactive",
	Short:   "Enter interactive mode",
	Args:    cobra.NoArgs,
	Aliases: []string{"i"},
	RunE: func(cmd *cobra.Command, args []string) error {
		u := uc()
		ctx, err := u.CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		p := tea.NewProgram(
			interactive.Model(u, ctx),
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		)

		if debug {
			f, err := tea.LogToFile("debug.log", "debug")
			if err != nil {
				return fmt.Errorf("open debug log: %w", err)
			}
			defer f.Close()
		}
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("tui exited: %w", err)
		}
		return nil
	},
}

func init() {
	Interactive.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug mode")
	RootCmd.AddCommand(Interactive)
}
