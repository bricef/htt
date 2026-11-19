//go:build windows

package cli

import "golang.org/x/sys/windows"

// openFile opens the file at path with the program Windows opens such
// files with, a web browser for a page, through ShellExecute. That
// starts it on its own, outside htt gui's console, so Ctrl-C there
// doesn't reach it.
func openFile(path string) error {
	return windows.ShellExecute(0, nil, windows.StringToUTF16Ptr(path), nil, nil, windows.SW_SHOWNORMAL)
}
