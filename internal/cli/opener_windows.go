//go:build windows

package cli

import "golang.org/x/sys/windows"

// openURL opens url in the default browser, through ShellExecute. That
// starts it on its own, outside htt gui's console, so Ctrl-C there
// doesn't reach it.
func openURL(url string) error {
	return windows.ShellExecute(0, nil, windows.StringToUTF16Ptr(url), nil, nil, windows.SW_SHOWNORMAL)
}
