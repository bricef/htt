package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tuipkg "github.com/bricef/htt/internal/tui"
	"github.com/bricef/htt/internal/storage"
	"github.com/bricef/htt/internal/usecase"
	"github.com/bricef/htt/internal/vars"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
)

// tuiEnv isolates one TUI test by routing viper paths to a fresh temp dir.
// TUI tests run serially because viper is global state.
type tuiEnv struct {
	t       *testing.T
	dataDir string
	model   tea.Model
}

func newTUIEnv(t *testing.T) *tuiEnv {
	t.Helper()
	dataDir := t.TempDir()

	viper.Set(vars.ConfigKeyDataDir, dataDir)
	viper.Set(vars.ConfigKeyTrackerDir, dataDir)
	viper.Set(vars.ConfigKeyDisableColor, true)

	return &tuiEnv{t: t, dataDir: dataDir}
}

// seedContext writes a context file with the given lines (one task per line).
func (e *tuiEnv) seedContext(name string, lines ...string) {
	e.t.Helper()
	if err := os.MkdirAll(e.dataDir, 0755); err != nil {
		e.t.Fatal(err)
	}
	path := filepath.Join(e.dataDir, name+".txt")
	content := ""
	for _, line := range lines {
		content += line + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		e.t.Fatal(err)
	}
}

// seedCurrentContext writes the current-context pointer file.
func (e *tuiEnv) seedCurrentContext(name string) {
	e.t.Helper()
	if err := os.MkdirAll(e.dataDir, 0755); err != nil {
		e.t.Fatal(err)
	}
	path := filepath.Join(e.dataDir, vars.DefaultContextFileName)
	if err := os.WriteFile(path, []byte(name), 0644); err != nil {
		e.t.Fatal(err)
	}
}

// start builds the TUI model rooted at the named context and sends the
// initial WindowSizeMsg. It expects the named context file to already exist.
func (e *tuiEnv) start(contextName string) {
	e.t.Helper()
	repo := storage.NewFileRepository(e.dataDir)
	uc := usecase.New(repo)
	ctx, err := repo.Context(contextName)
	if err != nil {
		e.t.Fatalf("Context(%q): %v", contextName, err)
	}
	e.model = tuipkg.Model(uc, ctx)
	e.send(tea.WindowSizeMsg{Width: 120, Height: 40})
}

// send dispatches a tea.Msg into the model.
func (e *tuiEnv) send(msg tea.Msg) {
	e.t.Helper()
	m, _ := e.model.Update(msg)
	e.model = m
}

// press sends a single tea.KeyMsg constructed from the given rune.
func (e *tuiEnv) press(r rune) {
	e.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
}

// pressKey sends a structural key like Enter, Esc, Up, Down, etc.
func (e *tuiEnv) pressKey(k tea.KeyType) {
	e.send(tea.KeyMsg{Type: k})
}

// type_ types each rune of s as a key press.
func (e *tuiEnv) type_(s string) {
	for _, r := range s {
		e.press(r)
	}
}

// view returns the model's rendered output with ANSI escapes stripped.
func (e *tuiEnv) view() string {
	return stripANSI(e.model.View())
}

// readData reads a file under the test data dir.
func (e *tuiEnv) readData(rel string) string {
	e.t.Helper()
	p := filepath.Join(e.dataDir, rel)
	b, err := os.ReadFile(p)
	if err != nil {
		e.t.Fatalf("could not read %s: %v", p, err)
	}
	return string(b)
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func assertViewContains(t *testing.T, view, needle string) {
	t.Helper()
	if !strings.Contains(view, needle) {
		t.Errorf("view should contain %q, got:\n%s", needle, view)
	}
}

func assertViewNotContains(t *testing.T, view, needle string) {
	t.Helper()
	if strings.Contains(view, needle) {
		t.Errorf("view should NOT contain %q, got:\n%s", needle, view)
	}
}
