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
)

var binaryPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "htt-e2e-bin-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not create temp dir for test binary:", err)
		os.Exit(1)
	}

	binaryPath = filepath.Join(tmp, "htt")

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

type env struct {
	t       *testing.T
	home    string
	dataDir string
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

type runResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func (e *env) run(args ...string) runResult {
	e.t.Helper()
	args = append([]string{"--no-color"}, args...)
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = []string{
		"HOME=" + e.home,
		"PATH=" + os.Getenv("PATH"),
	}
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
	r := e.run(args...)
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

func assertEqual(t *testing.T, label, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s mismatch\n got: %q\nwant: %q", label, got, want)
	}
}
