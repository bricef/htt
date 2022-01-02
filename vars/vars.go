package vars

import (
	"github.com/spf13/viper"
)

var (
	// Fixed values that I don't want stringly typed
	ConfigFileName = "config"
	NoColorFlag    = "no-color"

	// Config variable names for viper
	// A little more verbose than using directly, but
	// we get the compiler and IDE to help with completion
	// and checking. The strings will match the expected
	// key names in the config yaml file.
	ConfigKeyTrackerDir   = "tracker_path"
	ConfigKeyConfigPath   = "config_path"
	ConfigKeyDataDir      = "data_path"
	ConfigKeyRemoteName   = "remote_name"
	ConfigKeyRepoURL      = "backing_repo_url"
	ConfigKeyFilePatterns = "file_match_patterns"
	ConfigKeyAuthorName   = "author_name"
	ConfigKeyAuthorEmail  = "author_email"
	ConfigKeySSHKeyFile   = "ssh_key_file"
	ConfigKeyDisableColor = NoColorFlag

	// Default values
	DefaultContext         = "todo"
	DefaultDoneFileName    = "done"
	DefaultHomeDir         = ".htt"
	DefaultDataDirName     = "data"
	DefaultRemoteName      = "origin"
	DefaultTimelogDirName  = "timelogs"
	DefaultFileExtension   = ".txt"
	DefaultContextFileName = ".context"
	DefaultCommitterName   = "Hyper Time Tracker"
	DefaultCommitterEmail  = "htt@fractallambda.com"
	DefaultFilePatterns    = []string{"*" + DefaultFileExtension, ".context", DefaultTimelogDirName, ".gitignore"}
)

// Get a config variale by name
// because vars.Get is shorter than viper.GetString
func Get(name string) string {
	return viper.GetString(name)
}

func GetBool(name string) bool {
	return viper.GetBool(name)
}

func GetStringSlice(name string) []string {
	return viper.GetStringSlice(name)
}
