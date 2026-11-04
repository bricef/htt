package cli

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/bricef/htt/internal/update"
	"github.com/bricef/htt/internal/vars"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// noticeWait is how long a command waits, after it has finished, for a
// check still in flight. Only the day's checking run can wait.
const noticeWait = time.Second

var (
	updateOnce  sync.Once
	updateCheck *update.Check // nil when no check was allowed this run
)

// startUpdateCheck runs before every command (the root command's
// PersistentPreRun): it starts at most one check per process, and only
// when updateCheckAllowed says so. The REPL runs each line as a command,
// so the once matters.
func startUpdateCheck(cmd *cobra.Command) {
	updateOnce.Do(func() {
		interactive := cmd == Interactive || isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd())
		if !updateCheckAllowed(cmd, updateChecksEnabled(), interactive) {
			return
		}
		updateCheck = update.Start(update.Config{
			Current:   update.CurrentVersion(),
			StatePath: update.DefaultStatePath(),
		})
	})
}

// updateChecksEnabled reads the check_for_updates setting (default true).
func updateChecksEnabled() bool {
	return vars.GetBool(vars.ConfigKeyCheckForUpdates)
}

// updateCheckAllowed decides whether this run may check for updates:
// the config allows it, someone is watching (the TUI, or a terminal on
// stderr, where the notice goes), and the command isn't shell
// completion or help, which must never reach the network.
func updateCheckAllowed(cmd *cobra.Command, enabled, interactive bool) bool {
	if !enabled || !interactive {
		return false
	}
	for c := cmd; c != nil; c = c.Parent() {
		switch c.Name() {
		case "completion", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd, "help":
			return false
		}
	}
	return true
}

// PrintUpdateNotice writes the update notice, if this run's check found
// a newer release, to w (main passes stderr). It waits up to noticeWait
// for a check still in flight and gives up silently after that; any
// failure to check prints nothing.
func PrintUpdateNotice(w io.Writer) {
	if updateCheck == nil || !updateCheck.Wait(noticeWait) {
		return
	}
	if r, ok := updateCheck.Newer(); ok {
		_, _ = fmt.Fprintf(w, "\n%s\n", r.Notice())
	}
}

// tuiUpdateNotice gives the TUI the one-line notice once the check has
// finished ("" when there's nothing to say). Failures go to the TUI log.
func tuiUpdateNotice(logger interface{ Warn(string, ...any) }) func() string {
	return func() string {
		if updateCheck == nil {
			return ""
		}
		<-updateCheck.Done()
		if err := updateCheck.Err(); err != nil {
			logger.Warn("update check failed", "err", err)
		}
		if r, ok := updateCheck.Newer(); ok {
			return r.Short()
		}
		return ""
	}
}

// version is what htt --version prints.
func version() string {
	if v := update.CurrentVersion(); v != "" {
		return v
	}
	return "unknown (development build)"
}
