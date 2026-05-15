package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	cli "github.com/bricef/htt/internal/cli"
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
	configDir := filepath.Join(homeDir, vars.DefaultConfigDir)
	dataDir := filepath.Join(homeDir, vars.DefaultDataDir, vars.DefaultDataDirName)

	defaultConfigfilepath := filepath.Join(configDir, vars.ConfigFileName+".yaml")

	viper.SetDefault(vars.ConfigKeyTrackerDir, dataDir)
	viper.SetDefault(vars.ConfigKeyConfigPath, defaultConfigfilepath)
	viper.SetDefault(vars.ConfigKeyDataDir, dataDir)
	viper.SetDefault(vars.ConfigKeyRemoteName, vars.DefaultRemoteName)
	viper.SetDefault(vars.ConfigKeyFilePatterns, vars.DefaultFilePatterns)
	viper.SetDefault(vars.ConfigKeyAuthorName, vars.DefaultCommitterName)
	viper.SetDefault(vars.ConfigKeyAuthorEmail, vars.DefaultCommitterEmail)
	viper.SetDefault(vars.ConfigKeySSHKeyFile, filepath.Join(homeDir, ".ssh", "id_rsa.pub"))

	// viper.WriteConfigAs("dump.yaml")

	viper.SetConfigName(vars.ConfigFileName)
	viper.AddConfigPath(".")
	viper.AddConfigPath(configDir)

	// global flags
	var nocolor = false
	cli.RootCmd.PersistentFlags().BoolVarP(&nocolor, vars.NoColorFlag, "", false, "Disable color output for coloured commands")
	viper.BindPFlag(vars.NoColorFlag, cli.RootCmd.PersistentFlags().Lookup(vars.NoColorFlag))

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config not found.
			fmt.Printf("Could not find a configuration file.\nCreating a default file at %s.\n", defaultConfigfilepath)
			utils.EnsurePath(defaultConfigfilepath)
			err = viper.WriteConfigAs(defaultConfigfilepath)
			if err != nil {
				utils.Fatal("Failed to write default config file. ", err)
			}
		} else {
			// Config file was found but another error was produced
			utils.Fatal("An error occurred while loading the config. ", err)
		}
	}

	if err = cli.RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "❌", err)
		os.Exit(1)
	}
}
