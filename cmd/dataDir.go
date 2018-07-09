package cmd

import (
	"fmt"

	"github.com/hypotheticalco/tracker-client/vars"
	"github.com/spf13/cobra"
)

var dataDir = &cobra.Command{
	Use:   "dataDir",
	Short: "Outputs the currently configured datadir",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(vars.Get(vars.ConfigKeyDataDir))
	},
}

func init() {
	RootCmd.AddCommand(dataDir)
}
