// Package architecture asserts the dependency direction between internal
// packages after the Step 11 rename. It owns no production code, so it
// lives under test/ with the other suites.
package architecture

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const modulePath = "github.com/bricef/htt"

// TestArchitecture_DependencyDirections is the structural invariant the
// refactor promises:
//
//   - domain imports nothing from storage, cli, tui
//   - storage may import domain
//   - tui imports storage, domain — but NOT cli
//   - gui imports domain — but NOT cli, tui, storage or the config
//   - cli imports tui (to register the `interactive` subcommand), gui
//     (for `htt gui`) and everything below. cli → tui and cli → gui are
//     one-way: the CLI is the program's entry point and may launch the
//     TUI or the GUI as a mode.
//
// If a future change pulls one of these boundaries the wrong way, this test
// fails and forces the discussion before the dependency lands.
func TestArchitecture_DependencyDirections(t *testing.T) {
	forbidden := map[string][]string{
		"internal/domain": {
			"internal/storage",
			"internal/cli",
			"internal/tui",
			"internal/utils",
		},
		"internal/storage": {
			"internal/cli",
			"internal/tui",
		},
		// Date phrases are a leaf the CLI and TUI share.
		"internal/dates": {
			"internal/cli",
			"internal/tui",
			"internal/domain",
			"internal/storage",
			"internal/utils",
			"internal/vars",
			"internal/update",
		},
		// The update check is self-contained: it knows GitHub and the
		// cache file, nothing about tasks or config.
		"internal/update": {
			"internal/cli",
			"internal/tui",
			"internal/domain",
			"internal/storage",
			"internal/utils",
			"internal/vars",
		},
		// The editor launcher is a leaf the CLI and TUI share: it
		// builds the editor's command and leaves running it to them.
		"internal/editor": {
			"internal/cli",
			"internal/tui",
			"internal/domain",
			"internal/storage",
			"internal/utils",
			"internal/vars",
			"internal/update",
			"internal/dates",
			"internal/parseutils",
		},
		// The GUI's server reads through the domain's interfaces: the
		// CLI hands it a timelog repository, the categories and a clock,
		// so it needs neither the CLI, the config nor storage. cli may
		// import gui, to run it as `htt gui`.
		"internal/gui": {
			"internal/cli",
			"internal/tui",
			"internal/storage",
			"internal/vars",
			"internal/utils",
			"internal/update",
		},
		"internal/tui": {
			"internal/cli",
			// utils' Fatal helpers exit the process, and its editor
			// helper runs the editor on the terminal, which the TUI
			// owns; the TUI hands internal/editor's command to Bubble
			// Tea instead.
			"internal/utils",
		},
	}

	for pkg, blocked := range forbidden {
		imports := scanImports(t, pkg)
		for _, b := range blocked {
			fullBlocked := modulePath + "/" + b
			for _, imp := range imports {
				if imp == fullBlocked {
					t.Errorf("%s must not import %s (architectural boundary)", pkg, b)
				}
			}
		}
	}
}

// TestArchitecture_TUIDoesNotUseTheGlobalLogger pins where TUI
// diagnostics go. The TUI owns the terminal while it runs, and the
// process-wide log package writes to stderr by default, so a stray
// log.Printf would scribble over the screen. The TUI logs through the
// *slog.Logger handed to tui.Model instead (htt interactive --log-file).
func TestArchitecture_TUIDoesNotUseTheGlobalLogger(t *testing.T) {
	for _, imp := range scanImports(t, "internal/tui") {
		if imp == "log" {
			t.Errorf("internal/tui must not import log; log through the model's *slog.Logger")
		}
	}
}

// scanImports walks the files in repo-relative pkg path and returns the
// deduplicated list of imported package paths.
func scanImports(t *testing.T, pkg string) []string {
	t.Helper()

	repoRoot := findRepoRoot(t)
	dir := filepath.Join(repoRoot, pkg)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	seen := map[string]bool{}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s/%s: %v", pkg, name, err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			seen[path] = true
		}
	}

	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	return out
}

// findRepoRoot walks up from cwd until it finds go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod walking up from test dir")
		}
		dir = parent
	}
}
