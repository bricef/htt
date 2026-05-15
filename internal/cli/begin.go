package cli

import (
	"fmt"

	"github.com/bricef/htt/internal/domain"
	"github.com/spf13/cobra"
)

var Begin = &cobra.Command{
	Use:     "workon [item number]",
	Short:   "Log that work has began on numbered item in the current context.",
	Args:    cobra.MinimumNArgs(1),
	Aliases: []string{"wo"},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := repo().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		t, err := ctx.GetTaskByStrId(args[0])
		if err != nil {
			return fmt.Errorf("find task: %w", err)
		}
		// Make a fresh copy so the ts: annotation Timelog.Append adds
		// stays in the timelog and doesn't bleed back into the
		// context's in-memory state. Legacy behaviour was equivalent
		// (ctx was never saved after the AddEntry call), but the copy
		// makes the boundary explicit.
		entry, err := domain.NewTask(t.Raw)
		if err != nil {
			return fmt.Errorf("copy task: %w", err)
		}
		return appendToday(entry)
	},
}

func init() {
	RootCmd.AddCommand(Begin)
}
