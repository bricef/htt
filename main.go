package main

import (
	"fmt"
	"log"
	"os"
	"path"

	commands "github.com/hypotheticalco/tracker-client/cmd"
	vars "github.com/hypotheticalco/tracker-client/vars"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

func main() {
	// We need the home directory
	homeDir, err := homedir.Dir()
	if err != nil {
		log.Fatal(err)
	}

	// default values
	trackerDir := path.Join(homeDir, vars.DefaultHomeDir, vars.ProductName)

	viper.SetDefault(vars.ConfigKeyTrackerDir, trackerDir)
	viper.SetDefault(vars.ConfigKeyConfigPath, path.Join(trackerDir, vars.ConfigFileName+".yaml"))
	viper.SetDefault(vars.ConfigKeyDataDir, path.Join(trackerDir, vars.DefaultDataDirName))
	viper.SetDefault(vars.ConfigKeyRemoteName, vars.DefaultRemoteName)
	viper.SetDefault(vars.ConfigKeyTodoFileName, vars.DefaultTodoFileName)

	// viper.WriteConfigAs("dump.yaml")

	viper.SetConfigName(vars.ConfigFileName)
	viper.AddConfigPath(".")
	viper.AddConfigPath(trackerDir)

	err = viper.ReadInConfig()
	if err != nil {
		fmt.Println("Can't read config:", err)
		os.Exit(1)
	}

	if err = commands.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
