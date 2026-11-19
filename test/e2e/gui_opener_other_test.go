//go:build !unix

package e2e

import "testing"

func TestGUI_CtrlCLeavesTheBrowserRunning(t *testing.T) {
	t.Skip("a Unix matter: there the terminal's Ctrl-C goes to htt gui's process group; " +
		"on Windows htt opens the browser with ShellExecute, which starts it outside htt's console")
}
