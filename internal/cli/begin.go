package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Begin = &cobra.Command{
	Use:   "workon [item number]",
	Short: "Log that work has begun on numbered item in the current context.",
	Long: `Log the task with that number in the current context as a time-log
entry, so you don't retype it: its text, tags included, so its
+project picks its category. Its id: and p: tags, which link subtasks
to their parents, are left out.

` + atHelp + `

Example, for a task you started at 9:30:
  htt workon 3 --at 9:30`,
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
		// Make a fresh copy so the ts: annotation the timelog adds
		// stays in the timelog and doesn't bleed back into the
		// context's in-memory state. Legacy behaviour was equivalent
		// (ctx was never saved after the AddEntry call), but the copy
		// makes the boundary explicit. It leaves out id: and p:, which
		// link to nothing in the time log.
		entry, err := t.WithoutLinks()
		if err != nil {
			return fmt.Errorf("log task %s: %w", args[0], err)
		}
		_, err = logEntry(entry, logAt)
		return err
	},
}

func init() {
	Begin.Flags().Var(&logAt, "at", atFlagUsage)
	RootCmd.AddCommand(Begin)
}
