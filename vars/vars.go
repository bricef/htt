package vars

import (
	"github.com/spf13/viper"
)

var (
	ConfigFileName = "config"
	ProductName    = "tracker"

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

	// Default values
	DefaultContext         = "todo"
	DefaultDoneFileName    = "done"
	DefaultHomeDir         = ".hypothetical"
	DefaultDataDirName     = "data"
	DefaultRemoteName      = "origin"
	DefaultTimelogDirName  = "timelogs"
	DefaultFileExtension   = ".txt"
	DefaultContextFileName = ".context"
	DefaultCommitterName   = "Hypothetical Tracker"
	DefaultCommitterEmail  = "tracker-client@hypothetical.co"
	DefaultFilePatterns    = []string{"*" + DefaultFileExtension, ".context", DefaultTimelogDirName, ".gitignore"}
)

// Get a config variale by name
// because vars.Get is shorter than viper.GetString
func Get(name string) string {
	return viper.GetString(name)
}
