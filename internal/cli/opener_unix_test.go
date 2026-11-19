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
		t.Skip("macOS opens addresses with open, whatever else is on PATH")
	}
	bin := t.TempDir()
	for name, script := range scripts {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
}

// recordArgs is a script that writes its arguments, a line each, to the
// file at path.
func recordArgs(path string) string {
	return `printf '%s\n' "$@" > '` + path + `'`
}

const testURL = "http://127.0.0.1:41731/"

func TestOpenURL_SaysWhenTheOpenerFails(t *testing.T) {
	fakePrograms(t, map[string]string{"xdg-open": "exit 3"})
	if err := openURL(testURL); err == nil || err.Error() != "xdg-open exited 3" {
		t.Errorf("err = %v, want xdg-open exited 3", err)
	}
}

func TestOpenURL_AnOpenerStoppedBySignalIsntAFailure(t *testing.T) {
	// Stopped, the opener (which may be the browser) didn't fail to open
	// anything.
	fakePrograms(t, map[string]string{"xdg-open": "kill -TERM $$"})
	if err := openURL(testURL); err != nil {
		t.Errorf("err = %v, want none", err)
	}
}

func TestOpenURL_NoOpener(t *testing.T) {
	fakePrograms(t, nil)
	err := openURL(testURL)
	if err == nil || !strings.Contains(err.Error(), "found none of xdg-open, x-www-browser, www-browser, wslview, explorer.exe") {
		t.Errorf("err = %v", err)
	}
}

func TestOpenURL_GivesTheOpenerTheAddress(t *testing.T) {
	// Each opener, alone on PATH, is given the address, and only that.
	for _, name := range []string{"xdg-open", "x-www-browser", "www-browser", "wslview"} {
		t.Run(name, func(t *testing.T) {
			rec := filepath.Join(t.TempDir(), "args")
			fakePrograms(t, map[string]string{name: recordArgs(rec)})
			if err := openURL(testURL); err != nil {
				t.Fatalf("err = %v", err)
			}
			if got, err := os.ReadFile(rec); err != nil || string(got) != testURL+"\n" {
				t.Errorf("%s got %q (%v), want %q", name, got, err, testURL)
			}
		})
	}
}

func TestOpenURL_WSLWithoutWslu(t *testing.T) {
	// WSL without wslview: explorer.exe opens the address in Windows'
	// default browser, as it is; and its exit status, 1 when it has done
	// what it was asked, isn't a failure.
	rec := filepath.Join(t.TempDir(), "args")
	fakePrograms(t, map[string]string{"explorer.exe": recordArgs(rec) + "; exit 1"})
	if err := openURL(testURL); err != nil {
		t.Errorf("err = %v, want none", err)
	}
	if got, err := os.ReadFile(rec); err != nil || string(got) != testURL+"\n" {
		t.Errorf("explorer.exe got %q (%v), want %q", got, err, testURL)
	}
}
