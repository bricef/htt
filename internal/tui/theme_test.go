package tui

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestResolveTheme_DefaultIsTheOriginalPalette(t *testing.T) {
	// The colours the TUI was hard-coded with before themes: an
	// unconfigured TUI must look exactly as it did.
	want := map[string]string{
		"foreground": "#cfe0c3", "background": "#192b31", "selection": "#40798c",
		"accent": "#70a9a1", "muted": "#626262", "help": "#4a4a4a", "separator": "#3c3c3c",
	}
	for _, name := range []string{"", "default", "  Default "} {
		theme, err := ResolveTheme(name, nil)
		if err != nil {
			t.Fatalf("ResolveTheme(%q): %v", name, err)
		}
		for role, hex := range want {
			if got := string(*theme.role(role)); !strings.EqualFold(got, hex) {
				t.Errorf("ResolveTheme(%q).%s = %s, want %s", name, role, got, hex)
			}
		}
	}
}

func TestResolveTheme_NamedThemeThenOverrides(t *testing.T) {
	theme, err := ResolveTheme("MonoKai", map[string]string{
		"accent": "#abc",
		"Muted":  "205", // ANSI number; role names are case-insensitive
	})
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	monokai := themes["monokai"]
	if theme.Accent != "#abc" || theme.Muted != "205" {
		t.Errorf("overrides not applied: accent=%s muted=%s", theme.Accent, theme.Muted)
	}
	if theme.Foreground != monokai.Foreground || theme.Selection != monokai.Selection {
		t.Errorf("roles without an override should keep monokai's colours, got %+v", theme)
	}
	if themes["monokai"].Accent != monokai.Accent {
		t.Errorf("an override changed the built-in monokai theme itself")
	}
}

func TestResolveTheme_Errors(t *testing.T) {
	cases := []struct {
		name      string
		theme     string
		overrides map[string]string
		want      []string
	}{
		{"unknown theme", "solarised", nil, []string{`unknown theme "solarised"`, "monokai", "vscode"}},
		{"unknown role", "", map[string]string{"cursor": "#fff"}, []string{`unknown colour "cursor"`, "foreground, background"}},
		{"colour name", "", map[string]string{"accent": "red"}, []string{"tui.colors.accent", `invalid colour "red"`}},
		{"short hex", "", map[string]string{"accent": "#12345"}, []string{`invalid colour "#12345"`}},
		{"ANSI out of range", "", map[string]string{"accent": "256"}, []string{`invalid colour "256"`}},
		{"negative ANSI", "", map[string]string{"accent": "-1"}, []string{`invalid colour "-1"`}},
		{"unquoted hex (YAML comment)", "", map[string]string{"accent": ""}, []string{"empty colour", "quote hex colours"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ResolveTheme(tc.theme, tc.overrides)
			if err == nil {
				t.Fatalf("expected an error")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q should contain %q", err, want)
				}
			}
		})
	}
}

func TestThemeNames_DefaultFirstThenAlphabetical(t *testing.T) {
	names := ThemeNames()
	if len(names) != len(themes) || names[0] != DefaultThemeName {
		t.Fatalf("ThemeNames() = %v, want default first and every theme listed", names)
	}
	for i := 2; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Errorf("ThemeNames() not alphabetical after default: %v", names)
		}
	}
}

// luminance is the WCAG relative luminance of a #rrggbb colour.
func luminance(t *testing.T, hex string) float64 {
	t.Helper()
	if len(hex) != 7 || hex[0] != '#' {
		t.Fatalf("built-in colours should be #rrggbb, got %q", hex)
	}
	channel := func(s string) float64 {
		v, err := strconv.ParseUint(s, 16, 8)
		if err != nil {
			t.Fatalf("bad hex %q: %v", hex, err)
		}
		c := float64(v) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(hex[1:3]) + 0.7152*channel(hex[3:5]) + 0.0722*channel(hex[5:7])
}

func contrast(t *testing.T, a, b lipgloss.Color) float64 {
	la, lb := luminance(t, string(a)), luminance(t, string(b))
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func TestBuiltinThemes_CompleteAndReadable(t *testing.T) {
	// 3:1 is WCAG's minimum for large or bold text. The three pairs are
	// where the TUI draws text on a colour the theme chose: the
	// selected tab and task, the done tab and new-task input, and the
	// prompt and status line (checked against the theme's own
	// background, the closest stand-in for the terminal's).
	const minContrast = 3.0
	for _, name := range ThemeNames() {
		t.Run(name, func(t *testing.T) {
			theme := themes[name]
			for _, role := range roleNames {
				if _, err := parseColor(string(*theme.role(role))); err != nil {
					t.Errorf("%s: %v", role, err)
				}
			}
			pairs := []struct {
				what string
				a, b lipgloss.Color
			}{
				{"foreground on selection", theme.Foreground, theme.Selection},
				{"background on foreground", theme.Background, theme.Foreground},
				{"accent on background", theme.Accent, theme.Background},
			}
			for _, p := range pairs {
				if c := contrast(t, p.a, p.b); c < minContrast {
					t.Errorf("%s: contrast %.2f:1, want at least %.0f:1", p.what, c, minContrast)
				}
			}
		})
	}
}

// trueColor renders with 24-bit colour for the rest of the test, so
// the output carries the theme's exact colours.
func trueColor(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
}

var trueColorSGR = regexp.MustCompile(`(38|48);2;(\d+);(\d+);(\d+)`)

// hasColor reports whether output sets the #rrggbb colour as a 24-bit
// foreground (code 38) or background (code 48). Each channel may be
// off by one: lipgloss's colour conversion rounds.
func hasColor(output string, code int, hex string) bool {
	v, _ := strconv.ParseUint(hex[1:], 16, 32)
	want := [3]int{int(v >> 16), int(v>>8) & 0xff, int(v) & 0xff}
	for _, m := range trueColorSGR.FindAllStringSubmatch(output, -1) {
		if m[1] != strconv.Itoa(code) {
			continue
		}
		near := true
		for i := range want {
			got, _ := strconv.Atoi(m[i+2])
			if got < want[i]-1 || got > want[i]+1 {
				near = false
			}
		}
		if near {
			return true
		}
	}
	return false
}

func TestView_UsesTheModelsTheme(t *testing.T) {
	trueColor(t)
	theme, err := ResolveTheme("monokai", map[string]string{"selection": "#123456"})
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	seeded, repo := seedModel(t, "todo", "first task", "second task")
	m := Model(repo, seeded.context, Options{Theme: theme})
	m.width, m.height = 80, 24
	m.status = "a status message"

	view := m.View()
	for _, want := range []struct {
		what string
		code int
		hex  string
	}{
		{"selected row background (override)", 48, "#123456"},
		{"selected text (monokai foreground)", 38, "#f8f8f2"},
		{"status line (monokai accent)", 38, "#f92672"},
		{"borders (monokai muted)", 38, "#75715e"},
	} {
		if !hasColor(view, want.code, want.hex) {
			t.Errorf("view missing %s (%d;2;%s)", want.what, want.code, want.hex)
		}
	}
	if hasColor(view, 48, "#40798c") {
		t.Errorf("view still uses the default selection colour")
	}
}

func TestView_ZeroOptionsUseTheDefaultTheme(t *testing.T) {
	trueColor(t)
	m, _ := seedModel(t, "todo", "first task")
	m.width, m.height = 80, 24

	if view := m.View(); !hasColor(view, 48, "#40798c") {
		t.Errorf("zero Options should render the default theme's selection colour")
	}
}
