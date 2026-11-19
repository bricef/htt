package e2e

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// useFakeOpener puts a shell script first on htt's PATH, standing in for
// the program htt gui opens the browser with (xdg-open; open on macOS),
// and gives htt a TMPDIR of the test's own, which it returns: htt makes
// the file it opens there.
func (e *env) useFakeOpener(script string) (tmp string) {
	e.t.Helper()
	bin := e.t.TempDir()
	for _, name := range []string{"xdg-open", "open"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
			e.t.Fatal(err)
		}
	}
	tmp = e.t.TempDir()
	e.extraEnv = append(e.extraEnv, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "TMPDIR="+tmp)
	return tmp
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

// tokenOf is the token in the address htt gui printed.
func tokenOf(url string) string {
	base := strings.TrimSuffix(url, "/")
	return base[strings.LastIndex(base, "/")+1:]
}

func TestGUI_OpensTheBrowserWithoutTheToken(t *testing.T) {
	// A program's arguments are there for any user of the computer to
	// read (ps, /proc/<pid>/cmdline), and the token is the key to the
	// time log. So htt gui hands the opener a file, readable by you
	// alone, that sends the browser on to the address; and removes it
	// when it stops.
	if runtime.GOOS == "windows" {
		t.Skip("htt opens the browser with ShellExecute on Windows, which no program on PATH can stand in for")
	}
	e := newEnv(t)
	e.writeConfig("check_for_updates: false\n")
	rec := t.TempDir()
	tmp := e.useFakeOpener(`printf '%s\n' "$@" > '` + rec + `/args.tmp' && mv '` + rec + `/args.tmp' '` + rec + `/args'`)
	g := e.startGUIWith(nil)

	args := strings.Split(strings.TrimSuffix(waitForFile(t, filepath.Join(rec, "args")), "\n"), "\n")
	if strings.Contains(strings.Join(args, " "), tokenOf(g.url)) {
		t.Fatalf("the opener's arguments carry the token: %q", args)
	}
	if len(args) != 1 || !filepath.IsAbs(args[0]) {
		t.Fatalf("the opener got %q, want a file's path", args)
	}
	file, dir := args[0], filepath.Dir(args[0])
	if filepath.Dir(dir) != tmp {
		t.Errorf("the file is %s, want it in a directory of its own in TMPDIR (%s)", file, tmp)
	}
	for path, want := range map[string]fs.FileMode{file: 0o600, dir: 0o700} {
		if info, err := os.Stat(path); err != nil || info.Mode().Perm() != want {
			t.Errorf("%s: %v %v, want mode %v", path, info.Mode(), err, want)
		}
	}
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<meta http-equiv="refresh" content="0; url=` + g.url + `">`,
		`location.replace("` + g.url + `")`,
		`<a href="` + g.url + `">`,
	} {
		assertContains(t, "the redirect file", string(body), want)
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
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("htt gui stopped, and %s is still there (%v)", dir, err)
	}
}
