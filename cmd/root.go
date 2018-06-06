package cmd

import (
	"fmt"
	"os"
	"path"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "ht",
	Short: "Hypothetical tracker is a todo list manager and time tracker",
	Run: func(cmd *cobra.Command, args []string) {
		println(cmd.Short)
		println()
		// println(cmd.Long)
		cmd.Usage()
	},
}

var cfgFile = ""

// Execute the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "configfile", "", "absolute path to config file (default is $HOME/.hypothetical/tracker.yaml)")
}

func initConfig() {
	// We need the home directory
	homeDir, err := homedir.Dir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// default values
	appPath := path.Join(homeDir, ".hypothetical", "tracker")

	viper.AddConfigPath(".")
	viper.SetConfigName("config")

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(appPath)
	}

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Can't read config:", err)
		os.Exit(1)
	}

	rootCmd.AddCommand(cmdTodo)
	rootCmd.AddCommand(cmdTracking)

}
