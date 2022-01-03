package main

import (
	"fmt"
	"log"
	"os"
	"path"

	commands "github.com/bricef/htt/internal/cmd"
	"github.com/bricef/htt/internal/utils"
	vars "github.com/bricef/htt/internal/vars"
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
	defaultConfigfilepath := path.Join(trackerDir, vars.ConfigFileName+".yaml")

	viper.SetDefault(vars.ConfigKeyTrackerDir, trackerDir)
	viper.SetDefault(vars.ConfigKeyConfigPath, defaultConfigfilepath)
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

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config not found.
			fmt.Println("Could not find a configuration file. Creating a default file at %v.", defaultConfigfilepath)
			viper.WriteConfigAs(defaultConfigfilepath)
		} else {
			// Config file was found but another error was produced
			utils.Fatal("An error occured while loading teh config. ", err)
		}
	}

	if err = commands.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
