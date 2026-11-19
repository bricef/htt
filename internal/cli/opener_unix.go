//go:build unix

package cli

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// openers are the programs that open a file with the desktop's program
// for it, in the order htt tries them, outside macOS: xdg-open, then
// Debian's alternatives, then WSL's wslview (from wslu), which turns a
// Linux path into a Windows one.
var openers = []string{"xdg-open", "x-www-browser", "www-browser", "wslview"}

// openFile opens the file at path with the program the desktop opens
// such files with, a web browser for a page: with open on macOS, else
// with the first of openers on PATH, else, on WSL without wslu, with
// Windows' explorer.exe, given the file's Windows path.
//
// The opener runs in a process group of its own, as Ctrl-C in the
// terminal goes to htt gui's, and the opener may be the browser itself
// (xdg-open starting one that wasn't running), which outlives htt gui.
// Its output (xdg-open is chatty) is dropped.
func openFile(path string) error {
	if runtime.GOOS == "darwin" {
		return runOpener(exec.Command("open", path))
	}
	for _, name := range openers {
		if _, err := exec.LookPath(name); err == nil {
			return runOpener(exec.Command(name, path))
		}
	}
	if explorer, err := exec.LookPath("explorer.exe"); err == nil {
		win, err := exec.Command("wslpath", "-w", path).Output()
		if err != nil {
			return fmt.Errorf("wslpath -w: %w", err)
		}
		return runOpener(exec.Command(explorer, strings.TrimSpace(string(win))))
	}
	return errors.New("found none of " + strings.Join(append(openers, "explorer.exe"), ", ") + " on PATH")
}

// runOpener runs the opener cmd in a process group of its own and waits
// for it. An opener that exits with an error status failed, except
// explorer.exe, which exits 1 when it has done what it was asked; one
// stopped by a signal didn't fail to open anything, and htt says nothing
// about it.
func runOpener(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	err := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return err // nil, or it didn't start
	}
	name := filepath.Base(cmd.Path)
	if !exit.Exited() || name == "explorer.exe" {
		return nil
	}
	return fmt.Errorf("%s exited %d", name, exit.ExitCode())
}
