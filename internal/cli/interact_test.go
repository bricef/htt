package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bricef/htt/internal/tui"
	"github.com/bricef/htt/internal/vars"
)

// readLog closes the logger's file and returns what it wrote.
func readLog(t *testing.T, path string, closeLog func() error) string {
	t.Helper()
	if err := closeLog(); err != nil {
		t.Fatalf("close log: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	return string(b)
}

func TestDefaultTUITheme_MatchesTheTUIDefault(t *testing.T) {
	// vars can't import tui (tui stays config-free), so the default
	// theme name written to a fresh config is duplicated; keep them
	// in step.
	if vars.DefaultTUITheme != tui.DefaultThemeName {
		t.Errorf("vars.DefaultTUITheme = %q, tui.DefaultThemeName = %q", vars.DefaultTUITheme, tui.DefaultThemeName)
	}
}

func TestTUILogger_NoFileDiscards(t *testing.T) {
	logger, closeLog, err := tuiLogger("", "debug")
	if err != nil {
		t.Fatalf("tuiLogger: %v", err)
	}
	if logger.Handler().Enabled(t.Context(), -100) {
		t.Errorf("logger without --log-file should discard every level")
	}
	if err := closeLog(); err != nil {
		t.Errorf("close: %v", err)
	}
}

func TestTUILogger_DefaultsToDebugAndHonoursLevel(t *testing.T) {
	cases := []struct {
		level string
		want  []string
		skip  []string
	}{
		{"debug", []string{"level=DEBUG", "level=INFO", "level=WARN", "level=ERROR"}, nil},
		{"warn", []string{"level=WARN", "level=ERROR"}, []string{"level=DEBUG", "level=INFO"}},
		{"ERROR", []string{"level=ERROR"}, []string{"level=DEBUG", "level=INFO", "level=WARN"}},
	}
	for _, tc := range cases {
		t.Run(tc.level, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tui.log")
			logger, closeLog, err := tuiLogger(path, tc.level)
			if err != nil {
				t.Fatalf("tuiLogger: %v", err)
			}
			logger.Debug("d")
			logger.Info("i")
			logger.Warn("w")
			logger.Error("e")
			got := readLog(t, path, closeLog)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("--log-level %s: log missing %q:\n%s", tc.level, want, got)
				}
			}
			for _, skip := range tc.skip {
				if strings.Contains(got, skip) {
					t.Errorf("--log-level %s: log should not contain %q:\n%s", tc.level, skip, got)
				}
			}
		})
	}
}

func TestTUILogger_AppendsToAPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tui.log")
	for _, msg := range []string{"first run", "second run"} {
		logger, closeLog, err := tuiLogger(path, "debug")
		if err != nil {
			t.Fatalf("tuiLogger: %v", err)
		}
		logger.Info(msg)
		if err := closeLog(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(b), `"first run"`) || !strings.Contains(string(b), `"second run"`) {
		t.Errorf("second open should append, got:\n%s", b)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("log file mode = %o, want 600", perm)
	}
}

func TestTUILogger_RejectsBadLevelBeforeCreatingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tui.log")
	_, _, err := tuiLogger(path, "loud")
	if err == nil || !strings.Contains(err.Error(), `invalid --log-level "loud"`) {
		t.Fatalf("err = %v, want invalid --log-level", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a rejected level should not create the log file (stat err = %v)", err)
	}
}

func TestTUILogger_UnopenablePathFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-dir", "tui.log")
	_, _, err := tuiLogger(path, "debug")
	if err == nil || !strings.Contains(err.Error(), "open --log-file") {
		t.Fatalf("err = %v, want an open --log-file error", err)
	}
}
