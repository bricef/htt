package main

import (
	"fmt"
	"log"
	"os"
	"path"

	commands "github.com/bricef/htt/cmd"
	vars "github.com/bricef/htt/vars"
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
	trackerDir := path.Join(homeDir, vars.DefaultHomeDir)

	viper.SetDefault(vars.ConfigKeyTrackerDir, trackerDir)
	viper.SetDefault(vars.ConfigKeyConfigPath, path.Join(trackerDir, vars.ConfigFileName+".yaml"))
	viper.SetDefault(vars.ConfigKeyDataDir, path.Join(trackerDir, vars.DefaultDataDirName))
	viper.SetDefault(vars.ConfigKeyRemoteName, vars.DefaultRemoteName)
	viper.SetDefault(vars.ConfigKeyFilePatterns, vars.DefaultFilePatterns)
	viper.SetDefault(vars.ConfigKeyAuthorName, vars.DefaultCommitterName)
	viper.SetDefault(vars.ConfigKeyAuthorEmail, vars.DefaultCommitterEmail)
	viper.SetDefault(vars.ConfigKeySSHKeyFile, path.Join(homeDir, ".ssh", "id_rsa.pub"))

	// viper.WriteConfigAs("dump.yaml")

	viper.SetConfigName(vars.ConfigFileName)
	viper.AddConfigPath(".")
	viper.AddConfigPath(trackerDir)

	// global flags
	var nocolor = false
	commands.RootCmd.PersistentFlags().BoolVarP(&nocolor, vars.NoColorFlag, "", false, "Disable color output for coloured commands")
	viper.BindPFlag(vars.NoColorFlag, commands.RootCmd.PersistentFlags().Lookup(vars.NoColorFlag))

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
