package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// RootCmd is the top level command for ht
var RootCmd = &cobra.Command{
	Use:   "htt",
	Short: "htt is a command line todo list manager and time tracker",
	// SilenceUsage stops Cobra from printing usage on every RunE error.
	// SilenceErrors lets main.go own the error format (❌ prefix).
	SilenceUsage:  true,
	SilenceErrors: true,
	// Runs before every subcommand; see update.go.
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		startUpdateCheck(cmd)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(cmd.Short)
		fmt.Println()
		return cmd.Usage()
	},
}

func init() {
	RootCmd.Version = version()
}

// Execute runs the command args names, the words after htt: main runs
// the command line through it, and the REPL each line. A bare - where
// todo's command goes is read as down first (minusAsDown), which cobra
// can't do itself.
func Execute(args []string) error {
	RootCmd.SetArgs(minusAsDown(args))
	return RootCmd.Execute()
}
