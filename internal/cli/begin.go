package cli

import (
	"fmt"

	"github.com/bricef/htt/internal/timelogs"
	"github.com/spf13/cobra"
)

var Begin = &cobra.Command{
	Use:     "workon [item number]",
	Short:   "Log that work has began on numbered item in the current context.",
	Args:    cobra.MinimumNArgs(1),
	Aliases: []string{"wo"},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := uc().CurrentContext()
		if err != nil {
			return fmt.Errorf("load current context: %w", err)
		}
		t, err := ctx.GetTaskByStrId(args[0])
		if err != nil {
			return fmt.Errorf("find task: %w", err)
		}
		timelogs.AddEntry(t)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(Begin)
}
