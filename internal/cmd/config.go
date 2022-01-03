package cmd

import (
	"fmt"

	"github.com/bricef/htt/internal/todo"
	"github.com/bricef/htt/internal/utils"
	"github.com/bricef/htt/internal/vars"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

var Config = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration.",
}

var Print = &cobra.Command{
	Use:   "yaml",
	Short: "Prints out the current configuration in YAML.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c := viper.AllSettings()
		bs, err := yaml.Marshal(c)
		utils.DieOnError("Could not get YAML config. ", err)
		fmt.Print(string(bs))
	},
}

var Directory = &cobra.Command{
	Use:   "where-data",
	Short: "Outputs the currently configured data directory.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(vars.Get(vars.ConfigKeyDataDir))
	},
}

var Context = &cobra.Command{
	Use:   "context",
	Short: "Output the current context.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(todo.GetCurrentContext())
	},
}

func init() {
	Config.AddCommand(Print)
	Config.AddCommand(Directory)
	Config.AddCommand(Context)

	RootCmd.AddCommand(Config)
}
