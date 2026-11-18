package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// today returns today's local date in YYYY-MM-DD format. Used by
// e2e assertions that expect Context.AddTask to have stamped each
// new task with a creation date — the stamp is part of the on-disk
// Raw so tests that compare file contents need to include it.
func today() string {
	return time.Now().Format("2006-01-02")
}

var binaryPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "htt-e2e-bin-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not create temp dir for test binary:", err)
		os.Exit(1)
	}

	binaryPath = filepath.Join(tmp, exe("htt"))

	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/htt")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmp)
		fmt.Fprintln(os.Stderr, "failed to build htt binary:")
		fmt.Fprintln(os.Stderr, string(out))
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

// exe is the file name of a program called name on this platform:
// Windows runs a program only if its file name ends in .exe.
func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// skipOnWindowsForTZ skips a test that sets the time zone htt runs in
// with TZ. Go reads TZ on Unix only; on Windows, htt runs in the
// system's time zone whatever TZ says.
func skipOnWindowsForTZ(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("sets htt's time zone with TZ, which Go ignores on Windows")
	}
}

// skipOnWindowsForShellEditor skips a test whose $EDITOR is a shell
// script, which Windows can't run.
func skipOnWindowsForShellEditor(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("$EDITOR is a shell script, which Windows can't run")
	}
}

type env struct {
	t        *testing.T
	home     string
	dataDir  string
	extraEnv []string // added to the binary's environment, e.g. HTTPS_PROXY
	workDir  string   // the binary's working directory; "" is the test's
}

func newEnv(t *testing.T) *env {
	t.Helper()
	home := t.TempDir()

	configDir := filepath.Join(home, ".config", "htt")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	return &env{
		t:       t,
		home:    home,
		dataDir: filepath.Join(home, ".htt", "data"),
	}
}

// homeEnv is the environment that makes the env's home htt's home.
// htt finds it in HOME on every platform. On Windows, Go's own
// os.UserHomeDir reads USERPROFILE, and os.UserCacheDir LOCALAPPDATA.
func (e *env) homeEnv() []string {
	home := []string{"HOME=" + e.home}
	if runtime.GOOS == "windows" {
		home = append(home, "USERPROFILE="+e.home, "LOCALAPPDATA="+e.cacheDir())
	}
	return home
}

// cacheDir is where htt keeps its cache, os.UserCacheDir, in the env's
// home.
func (e *env) cacheDir() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(e.home, "AppData", "Local")
	case "darwin":
		return filepath.Join(e.home, "Library", "Caches")
	default:
		return filepath.Join(e.home, ".cache")
	}
}

type runResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func (e *env) run(args ...string) runResult {
	e.t.Helper()
	return e.runWithInput("", args...)
}

// runWithInput runs htt with input piped to its stdin; "" leaves
// stdin unconnected, as run does.
func (e *env) runWithInput(input string, args ...string) runResult {
	e.t.Helper()
	args = append([]string{"--no-color"}, args...)
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(append(e.homeEnv(), "PATH="+os.Getenv("PATH")), e.extraEnv...)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	cmd.Dir = e.workDir
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		e.t.Fatalf("could not run htt: %v", err)
	}
	return runResult{
		stdout:   outBuf.String(),
		stderr:   errBuf.String(),
		exitCode: code,
	}
}

func (e *env) mustRun(args ...string) runResult {
	e.t.Helper()
	return e.mustRunWithInput("", args...)
}

// mustRunWithInput is runWithInput, failing the test on a non-zero exit.
func (e *env) mustRunWithInput(input string, args ...string) runResult {
	e.t.Helper()
	r := e.runWithInput(input, args...)
	if r.exitCode != 0 {
		e.t.Fatalf("htt %s exited %d\nstdout:\n%s\nstderr:\n%s",
			strings.Join(args, " "), r.exitCode, r.stdout, r.stderr)
	}
	return r
}

func (e *env) readData(rel string) string {
	e.t.Helper()
	p := filepath.Join(e.dataDir, rel)
	b, err := os.ReadFile(p)
	if err != nil {
		e.t.Fatalf("could not read %s: %v", p, err)
	}
	return string(b)
}

// writeData seeds a data file directly, for content htt itself
// wouldn't write (blank lines, hand edits).
func (e *env) writeData(rel, content string) {
	e.t.Helper()
	p := filepath.Join(e.dataDir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		e.t.Fatal(err)
	}
}

// writeConfig replaces the env's config.yaml.
func (e *env) writeConfig(content string) {
	e.t.Helper()
	p := filepath.Join(e.home, ".config", "htt", "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) dataExists(rel string) bool {
	_, err := os.Stat(filepath.Join(e.dataDir, rel))
	return err == nil
}

func assertContains(t *testing.T, label, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("%s: expected to contain %q, got:\n%s", label, needle, haystack)
	}
}

// assertInOrder checks that each needle appears in haystack, each
// one after the needle before it.
func assertInOrder(t *testing.T, label, haystack string, needles ...string) {
	t.Helper()
	rest := haystack
	for _, needle := range needles {
		i := strings.Index(rest, needle)
		if i < 0 {
			t.Errorf("%s: expected %q in this order, %q missing or out of order, got:\n%s",
				label, needles, needle, haystack)
			return
		}
		rest = rest[i+len(needle):]
	}
}

func assertEqual(t *testing.T, label, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s mismatch\n got: %q\nwant: %q", label, got, want)
	}
}
