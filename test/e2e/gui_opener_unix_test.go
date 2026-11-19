//go:build unix

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestGUI_CtrlCLeavesTheBrowserRunning(t *testing.T) {
	// The terminal's Ctrl-C goes to htt gui's process group. The opener
	// can be the browser itself (xdg-open starting one that wasn't
	// running), so it runs in a group of its own and lives on; and htt
	// doesn't say it couldn't open a browser.
	e := newEnv(t)
	e.writeConfig("check_for_updates: false\n")
	pidFile := filepath.Join(t.TempDir(), "pid")
	e.useFakeOpener(`echo $$ > '` + pidFile + `.tmp' && mv '` + pidFile + `.tmp' '` + pidFile + `'
exec sleep 60`)
	// htt in a group of its own, as a terminal runs a job.
	g := e.startGUIWith(func(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} })
	pid, err := strconv.Atoi(strings.TrimSpace(waitForFile(t, pidFile)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })

	if err := syscall.Kill(-g.cmd.Process.Pid, syscall.SIGINT); err != nil { // Ctrl-C
		t.Fatal(err)
	}
	select {
	case err := <-g.done:
		g.done <- err // for the cleanup
		if stdout, stderr := g.output(); err != nil || stderr != "" {
			t.Errorf("htt gui exited with %v after Ctrl-C\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("htt gui still running 10s after Ctrl-C")
	}
	if !running(pid) {
		t.Errorf("htt gui's Ctrl-C stopped the opener (pid %d) too", pid)
	}
}

// running reports whether process pid is alive: there, and not a
// zombie waiting to be reaped.
func running(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return true // no /proc (macOS): it's there
	}
	// The state follows the command's name, which is in parentheses.
	fields := strings.Fields(string(stat[strings.LastIndexByte(string(stat), ')')+1:]))
	return len(fields) > 0 && fields[0] != "Z"
}
