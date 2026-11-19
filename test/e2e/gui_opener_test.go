package e2e

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// useFakeOpener puts a shell script first on htt's PATH, standing in for
// the program htt gui opens the browser with (xdg-open; open on macOS).
func (e *env) useFakeOpener(script string) {
	e.t.Helper()
	bin := e.t.TempDir()
	for _, name := range []string{"xdg-open", "open"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
			e.t.Fatal(err)
		}
	}
	e.extraEnv = append(e.extraEnv, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// waitForFile waits for the file at path, which a fake opener writes,
// and returns what it holds.
func waitForFile(t *testing.T, path string) string {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		b, err := os.ReadFile(path)
		if err == nil {
			return string(b)
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %s after 10s: the opener didn't run", filepath.Base(path))
		}
	}
}

func TestGUI_OpensTheBrowserAtTheAddress(t *testing.T) {
	// htt gui hands the opener the address it printed, and nothing else;
	// stopped, it exits cleanly and says nothing about the opener.
	if runtime.GOOS == "windows" {
		t.Skip("htt opens the browser with ShellExecute on Windows, which no program on PATH can stand in for")
	}
	e := newEnv(t)
	e.writeConfig("check_for_updates: false\n")
	rec := t.TempDir()
	e.useFakeOpener(`printf '%s\n' "$@" > '` + rec + `/args.tmp' && mv '` + rec + `/args.tmp' '` + rec + `/args'`)
	g := e.startGUIWith(nil)

	args := strings.Split(strings.TrimSuffix(waitForFile(t, filepath.Join(rec, "args")), "\n"), "\n")
	if len(args) != 1 || args[0] != g.url {
		t.Errorf("the opener got %q, want the address htt gui printed, %q", args, g.url)
	}

	if err := g.cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-g.done:
		g.done <- err // for the cleanup
		if _, stderr := g.output(); err != nil || stderr != "" {
			t.Errorf("htt gui exited with %v after an interrupt; stderr: %s", err, stderr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("htt gui still running 10s after an interrupt")
	}
}

func TestGUI_SaysWhenItCantOpenABrowser(t *testing.T) {
	// An opener that fails: htt gui says so, and serves all the same.
	if runtime.GOOS == "windows" {
		t.Skip("htt opens the browser with ShellExecute on Windows, which no program on PATH can stand in for")
	}
	e := newEnv(t)
	e.writeConfig("check_for_updates: false\n")
	e.useFakeOpener("exit 3")
	g := e.startGUIWith(nil)
	var stderr string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if _, stderr = g.output(); strings.Contains(stderr, "\n") {
			break
		}
	}
	assertContains(t, "stderr", stderr, "htt gui: couldn't open a browser (")
	assertContains(t, "stderr", stderr, " exited 3); open the address above yourself")
	if code, _ := fetch(t, http.MethodGet, g.url, ""); code != http.StatusOK {
		t.Errorf("GET %s: %d, want 200", g.url, code)
	}
}
