package cmd

import (
	"fmt"

	"github.com/bricef/htt/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

var config = &cobra.Command{
	Use:   "config",
	Short: "Prints out the current configuration in YAML",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c := viper.AllSettings()
		bs, err := yaml.Marshal(c)
		utils.DieOnError("Could not get YAML config. ", err)
		fmt.Print(string(bs))
	},
}

func init() {
	RootCmd.AddCommand(config)
}
