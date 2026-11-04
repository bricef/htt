package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestUpdateChecksEnabled_ReadsTheConfigKey(t *testing.T) {
	// The README tells users to write check_for_updates: false.
	prev := viper.Get("check_for_updates")
	t.Cleanup(func() { viper.Set("check_for_updates", prev) })

	viper.Set("check_for_updates", false)
	if updateChecksEnabled() {
		t.Errorf("check_for_updates: false should turn the check off")
	}
	viper.Set("check_for_updates", true)
	if !updateChecksEnabled() {
		t.Errorf("check_for_updates: true should leave the check on")
	}
}

func TestUpdateCheckAllowed(t *testing.T) {
	// A stand-in command tree: cobra only adds its completion commands
	// when Execute runs, so build the shapes we need here.
	root := &cobra.Command{Use: "htt"}
	todo := &cobra.Command{Use: "todo"}
	add := &cobra.Command{Use: "add"}
	todo.AddCommand(add)
	completion := &cobra.Command{Use: "completion"}
	zsh := &cobra.Command{Use: "zsh"}
	completion.AddCommand(zsh)
	complete := &cobra.Command{Use: cobra.ShellCompRequestCmd}
	completeNoDesc := &cobra.Command{Use: cobra.ShellCompNoDescRequestCmd}
	help := &cobra.Command{Use: "help"}
	root.AddCommand(todo, completion, complete, completeNoDesc, help)

	cases := []struct {
		name                 string
		cmd                  *cobra.Command
		enabled, interactive bool
		want                 bool
	}{
		{"a command at a terminal", add, true, true, true},
		{"the root command", root, true, true, true},
		{"turned off in the config", add, false, true, false},
		{"no terminal (scripts, pipes, cron)", add, true, false, false},
		{"completion script", zsh, true, true, false},
		{"tab completion request", complete, true, true, false},
		{"tab completion request, no descriptions", completeNoDesc, true, true, false},
		{"help", help, true, true, false},
	}
	for _, tc := range cases {
		if got := updateCheckAllowed(tc.cmd, tc.enabled, tc.interactive); got != tc.want {
			t.Errorf("%s: updateCheckAllowed = %v, want %v", tc.name, got, tc.want)
		}
	}
}
