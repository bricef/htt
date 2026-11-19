//go:build unix

package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakePrograms makes PATH a directory holding only the given shell
// scripts, by name.
func fakePrograms(t *testing.T, scripts map[string]string) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		t.Skip("macOS opens files with open, whatever else is on PATH")
	}
	bin := t.TempDir()
	for name, script := range scripts {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
}

func TestOpenFile_SaysWhenTheOpenerFails(t *testing.T) {
	fakePrograms(t, map[string]string{"xdg-open": "exit 3"})
	if err := openFile("/tmp/open.html"); err == nil || err.Error() != "xdg-open exited 3" {
		t.Errorf("err = %v, want xdg-open exited 3", err)
	}
}

func TestOpenFile_AnOpenerStoppedBySignalIsntAFailure(t *testing.T) {
	// Stopped, the opener (which may be the browser) didn't fail to open
	// anything.
	fakePrograms(t, map[string]string{"xdg-open": "kill -TERM $$"})
	if err := openFile("/tmp/open.html"); err != nil {
		t.Errorf("err = %v, want none", err)
	}
}

func TestOpenFile_NoOpener(t *testing.T) {
	fakePrograms(t, nil)
	err := openFile("/tmp/open.html")
	if err == nil || !strings.Contains(err.Error(), "found none of xdg-open, x-www-browser, www-browser, wslview, explorer.exe") {
		t.Errorf("err = %v", err)
	}
}

func TestOpenFile_WSLWithoutWslu(t *testing.T) {
	// WSL without wslview: explorer.exe opens the file by its Windows
	// path, which wslpath gives; and its exit status, 1 when it has done
	// what it was asked, isn't a failure.
	rec := t.TempDir()
	fakePrograms(t, map[string]string{
		"wslpath":      `printf '%s\n' "$@" > '` + rec + `/wslpath'; printf '%s\n' '\\wsl.localhost\Ubuntu\tmp\htt-gui-1\open.html'`,
		"explorer.exe": `printf '%s\n' "$@" > '` + rec + `/explorer'; exit 1`,
	})
	if err := openFile("/tmp/htt-gui-1/open.html"); err != nil {
		t.Errorf("err = %v, want none", err)
	}
	for name, want := range map[string]string{
		"wslpath":  "-w\n/tmp/htt-gui-1/open.html\n",
		"explorer": `\\wsl.localhost\Ubuntu\tmp\htt-gui-1\open.html` + "\n",
	} {
		if got, err := os.ReadFile(filepath.Join(rec, name)); err != nil || string(got) != want {
			t.Errorf("%s got %q (%v), want %q", name, got, err, want)
		}
	}
}
