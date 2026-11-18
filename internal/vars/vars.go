package vars

import (
	"path/filepath"

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
	ConfigKeyTrackerDir      = "tracker_path"
	ConfigKeyConfigPath      = "config_path"
	ConfigKeyDataDir         = "data_path"
	ConfigKeyDisableColor    = NoColorFlag
	ConfigKeyTUITheme        = "tui.theme"  // a built-in theme name
	ConfigKeyTUIColors       = "tui.colors" // per-role colour overrides
	ConfigKeyCheckForUpdates = "check_for_updates"

	// The time log section: its categories (a list, since the first
	// match wins) and the category of entries none matches.
	ConfigKeyTimelog                = "timelog"
	ConfigKeyTimelogCategories      = "timelog.categories"
	ConfigKeyTimelogDefaultCategory = "timelog.default_category"

	// Default values
	DefaultContext         = "todo"
	DefaultDoneFileName    = "done"
	DefaultDataDir         = ".htt"
	DefaultDataDirName     = "data"
	DefaultConfigDir       = ".config/htt"
	DefaultTimelogDirName  = "timelogs"
	DefaultContextFileName = "current-context"
	DefaultTUITheme        = "default" // matches tui.DefaultThemeName
	DefaultTimelogCategory = "working" // matches domain.DefaultCategoryName
)

// Get a config variale by name
// because vars.Get is shorter than viper.GetString
func Get(name string) string {
	return viper.GetString(name)
}

// ConfigDir returns the config directory: the directory holding the
// config file in effect. That's the file viper read (config.yaml in
// the working directory, if there is one), or, when it read none (the
// first run), the directory of config_path, where htt writes the
// default file. config_path itself names the file, not the directory.
func ConfigDir() string {
	if f := viper.ConfigFileUsed(); f != "" {
		return filepath.Dir(f)
	}
	if p := Get(ConfigKeyConfigPath); p != "" {
		return filepath.Dir(p)
	}
	return ""
}

func GetBool(name string) bool {
	return viper.GetBool(name)
}

func GetStringMapString(name string) map[string]string {
	return viper.GetStringMapString(name)
}
