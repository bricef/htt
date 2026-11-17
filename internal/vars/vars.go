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
	ConfigKeyTrackerDir      = "tracker_path"
	ConfigKeyConfigPath      = "config_path"
	ConfigKeyDataDir         = "data_path"
	ConfigKeyDisableColor    = NoColorFlag
	ConfigKeyTUITheme        = "tui.theme"  // a built-in theme name
	ConfigKeyTUIColors       = "tui.colors" // per-role colour overrides
	ConfigKeyCheckForUpdates = "check_for_updates"

	// Default values
	DefaultContext         = "todo"
	DefaultDoneFileName    = "done"
	DefaultDataDir         = ".htt"
	DefaultDataDirName     = "data"
	DefaultConfigDir       = ".config/htt"
	DefaultTimelogDirName  = "timelogs"
	DefaultContextFileName = "current-context"
	DefaultTUITheme        = "default" // matches tui.DefaultThemeName
)

// Get a config variale by name
// because vars.Get is shorter than viper.GetString
func Get(name string) string {
	return viper.GetString(name)
}

func GetBool(name string) bool {
	return viper.GetBool(name)
}

func GetStringMapString(name string) map[string]string {
	return viper.GetStringMapString(name)
}
