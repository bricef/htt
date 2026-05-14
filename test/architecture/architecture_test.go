// Package architecture asserts the dependency direction between internal
// packages after the Step 11 rename. It owns no production code; lives
// under test/ to avoid colliding with internal/gcal.go's package main.
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
//   - domain imports nothing from storage, usecase, cli, tui
//   - storage may import domain
//   - usecase may import domain, storage
//   - tui imports usecase, storage, domain — but NOT cli
//   - cli imports tui (to register the `interactive` subcommand) and
//     everything below. cli → tui is one-way: the CLI is the program's
//     entry point and may launch the TUI as a mode.
//
// If a future change pulls one of these boundaries the wrong way, this test
// fails and forces the discussion before the dependency lands.
func TestArchitecture_DependencyDirections(t *testing.T) {
	forbidden := map[string][]string{
		"internal/domain": {
			"internal/storage",
			"internal/usecase",
			"internal/cli",
			"internal/tui",
			"internal/utils",
		},
		"internal/storage": {
			"internal/usecase",
			"internal/cli",
			"internal/tui",
		},
		"internal/usecase": {
			"internal/cli",
			"internal/tui",
		},
		"internal/tui": {
			"internal/cli",
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
