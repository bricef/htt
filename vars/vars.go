package vars

import (
	"github.com/spf13/viper"
)

const (
	ConfigFileName = "config"
	ProductName    = "tracker"

	// Config variable names for viper
	ConfigKeyTrackerDir   = "tracker_path"
	ConfigKeyConfigPath   = "config_path"
	ConfigKeyDataDir      = "data_path"
	ConfigKeyRemoteName   = "remote_name"
	ConfigKeyTodoFileName = "todo_file"
	ConfigKeyRepoURL      = "backing_repo_url"

	// Default values
	DefaultHomeDir      = ".hypothetical"
	DefaultDataDirName  = "data"
	DefaultRemoteName   = "origin"
	DefaultTodoFileName = "todo.txt"
)

// Get a config variale by name
func Get(name string) string {
	return viper.GetString(name)
}
