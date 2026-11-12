package tui

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/bricef/htt/internal/domain"
	lipgloss "github.com/charmbracelet/lipgloss"
)

// DefaultThemeName is the theme used when the config names none.
const DefaultThemeName = "default"

// Theme is the TUI's palette: one colour per role, each named for what
// it paints. The TUI never paints the screen background, so tabs and
// tasks that aren't selected use the terminal's own colours.
type Theme struct {
	Foreground lipgloss.Color // the title; text on the selected tab and task; the fill of the done tab and the new-task input
	Background lipgloss.Color // text drawn on that fill
	Selection  lipgloss.Color // the selected tab and task
	Accent     lipgloss.Color // the new-task prompt and the status line
	Muted      lipgloss.Color // borders, keys in the help bar, the input placeholder
	Help       lipgloss.Color // descriptions in the help bar
	Separator  lipgloss.Color // separators in the help bar

	// The parts of a task line, on every row but the selected one
	// (which keeps Foreground on Selection). Plain words keep the
	// terminal's colour.
	PriorityA  lipgloss.Color // (A)
	PriorityB  lipgloss.Color // (B)
	PriorityC  lipgloss.Color // (C)
	PlusTag    lipgloss.Color // +tag
	AtTag      lipgloss.Color // @tag
	HashTag    lipgloss.Color // #tag
	Annotation lipgloss.Color // key:value, such as due:2026-05-20
	Date       lipgloss.Color // the created-on and completed-on dates
}

// roleNames are the config names of the roles (tui.colors.<name>), in
// the order docs and errors list them.
var roleNames = []string{
	"foreground", "background", "selection", "accent", "muted", "help", "separator",
	"priority_a", "priority_b", "priority_c", "plus_tag", "at_tag", "hash_tag", "annotation", "date",
}

// role returns the field a config name sets, or nil for an unknown one.
func (t *Theme) role(name string) *lipgloss.Color {
	switch name {
	case "foreground":
		return &t.Foreground
	case "background":
		return &t.Background
	case "selection":
		return &t.Selection
	case "accent":
		return &t.Accent
	case "muted":
		return &t.Muted
	case "help":
		return &t.Help
	case "separator":
		return &t.Separator
	case "priority_a":
		return &t.PriorityA
	case "priority_b":
		return &t.PriorityB
	case "priority_c":
		return &t.PriorityC
	case "plus_tag":
		return &t.PlusTag
	case "at_tag":
		return &t.AtTag
	case "hash_tag":
		return &t.HashTag
	case "annotation":
		return &t.Annotation
	case "date":
		return &t.Date
	}
	return nil
}

// themes are the built-in palettes. default is the TUI's original
// colours, with task-line colours in the same spirit; the rest map each
// palette's canonical colours onto the roles. The one exception is
// solarized-light, whose canonical yellow, green and cyan fall just
// short of 3:1 on its own background; its priority_b, priority_c and
// hash_tag are those colours darkened until they reach it.
var themes = map[string]Theme{
	DefaultThemeName: {
		Foreground: "#cfe0c3", Background: "#192b31", Selection: "#40798c", Accent: "#70a9a1",
		Muted: "#626262", Help: "#4a4a4a", Separator: "#3c3c3c",
		PriorityA: "#ef6f6c", PriorityB: "#e5c07b", PriorityC: "#98c379",
		PlusTag: "#61afef", AtTag: "#c678dd", HashTag: "#56b6c2", Annotation: "#8a9ba0", Date: "#7f9196",
	},
	"monokai": {
		Foreground: "#f8f8f2", Background: "#272822", Selection: "#49483e", Accent: "#f92672",
		Muted: "#75715e", Help: "#75715e", Separator: "#3e3d32",
		PriorityA: "#f92672", PriorityB: "#fd971f", PriorityC: "#a6e22e",
		PlusTag: "#66d9ef", AtTag: "#ae81ff", HashTag: "#e6db74", Annotation: "#75715e", Date: "#75715e",
	},
	"vscode": { // Dark+
		Foreground: "#d4d4d4", Background: "#1e1e1e", Selection: "#264f78", Accent: "#569cd6",
		Muted: "#858585", Help: "#858585", Separator: "#3c3c3c",
		PriorityA: "#f44747", PriorityB: "#ce9178", PriorityC: "#6a9955",
		PlusTag: "#569cd6", AtTag: "#c586c0", HashTag: "#4ec9b0", Annotation: "#9cdcfe", Date: "#858585",
	},
	"vscode-light": { // Light+
		Foreground: "#000000", Background: "#ffffff", Selection: "#add6ff", Accent: "#007acc",
		Muted: "#6e7681", Help: "#6e7681", Separator: "#e5e5e5",
		PriorityA: "#cd3131", PriorityB: "#795e26", PriorityC: "#008000",
		PlusTag: "#0000ff", AtTag: "#af00db", HashTag: "#267f99", Annotation: "#001080", Date: "#6e7681",
	},
	"dracula": {
		Foreground: "#f8f8f2", Background: "#282a36", Selection: "#44475a", Accent: "#bd93f9",
		Muted: "#6272a4", Help: "#6272a4", Separator: "#44475a",
		PriorityA: "#ff5555", PriorityB: "#ffb86c", PriorityC: "#50fa7b",
		PlusTag: "#8be9fd", AtTag: "#ff79c6", HashTag: "#bd93f9", Annotation: "#f1fa8c", Date: "#6272a4",
	},
	"nord": {
		Foreground: "#d8dee9", Background: "#2e3440", Selection: "#434c5e", Accent: "#88c0d0",
		Muted: "#616e88", Help: "#4c566a", Separator: "#3b4252",
		PriorityA: "#bf616a", PriorityB: "#ebcb8b", PriorityC: "#a3be8c",
		PlusTag: "#81a1c1", AtTag: "#b48ead", HashTag: "#88c0d0", Annotation: "#d08770", Date: "#8fbcbb",
	},
	"solarized-dark": {
		Foreground: "#93a1a1", Background: "#002b36", Selection: "#073642", Accent: "#268bd2",
		Muted: "#586e75", Help: "#586e75", Separator: "#073642",
		PriorityA: "#dc322f", PriorityB: "#b58900", PriorityC: "#859900",
		PlusTag: "#268bd2", AtTag: "#d33682", HashTag: "#2aa198", Annotation: "#6c71c4", Date: "#839496",
	},
	"solarized-light": {
		Foreground: "#586e75", Background: "#fdf6e3", Selection: "#eee8d5", Accent: "#268bd2",
		Muted: "#93a1a1", Help: "#93a1a1", Separator: "#eee8d5",
		PriorityA: "#dc322f", PriorityB: "#ad8300", PriorityC: "#7f9200",
		PlusTag: "#268bd2", AtTag: "#d33682", HashTag: "#259a91", Annotation: "#6c71c4", Date: "#657b83",
	},
	"gruvbox": { // dark
		Foreground: "#ebdbb2", Background: "#282828", Selection: "#504945", Accent: "#fabd2f",
		Muted: "#928374", Help: "#928374", Separator: "#3c3836",
		PriorityA: "#fb4934", PriorityB: "#fabd2f", PriorityC: "#b8bb26",
		PlusTag: "#83a598", AtTag: "#d3869b", HashTag: "#8ec07c", Annotation: "#fe8019", Date: "#928374",
	},
}

// ThemeNames lists the built-in themes: default first, then the rest
// alphabetically.
func ThemeNames() []string {
	names := slices.Sorted(maps.Keys(themes))
	names = slices.DeleteFunc(names, func(n string) bool { return n == DefaultThemeName })
	return append([]string{DefaultThemeName}, names...)
}

// RoleNames lists the colour roles a config can set, in display order.
func RoleNames() []string {
	return slices.Clone(roleNames)
}

// ResolveTheme builds the theme a config asks for: the named built-in
// theme (default when name is empty), with each override applied on
// top. Theme and role names are case-insensitive. A colour is hex
// (#rgb or #rrggbb) or an ANSI 256-colour number (0-255).
func ResolveTheme(name string, overrides map[string]string) (Theme, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		key = DefaultThemeName
	}
	theme, ok := themes[key]
	if !ok {
		return Theme{}, fmt.Errorf("unknown theme %q (want one of: %s)", name, strings.Join(ThemeNames(), ", "))
	}
	for _, role := range slices.Sorted(maps.Keys(overrides)) {
		field := theme.role(strings.ToLower(role))
		if field == nil {
			return Theme{}, fmt.Errorf("unknown colour %q in tui.colors (want one of: %s)", role, strings.Join(roleNames, ", "))
		}
		c, err := parseColor(overrides[role])
		if err != nil {
			return Theme{}, fmt.Errorf("tui.colors.%s: %w", role, err)
		}
		*field = c
	}
	return theme, nil
}

var hexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// parseColor validates one configured colour. An empty value is almost
// always an unquoted hex colour, which YAML reads as a comment.
func parseColor(value string) (lipgloss.Color, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", errors.New(`empty colour; quote hex colours in YAML ("#f92672"), since an unquoted # starts a comment`)
	}
	if hexColor.MatchString(v) {
		return lipgloss.Color(v), nil
	}
	if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 255 {
		return lipgloss.Color(v), nil
	}
	return "", fmt.Errorf("invalid colour %q (want #rgb, #rrggbb or an ANSI colour number 0-255)", value)
}

// styles are the lipgloss styles the TUI renders with, derived once
// from a theme.
type styles struct {
	theme      Theme
	task       lipgloss.Style // the selected task row
	status     lipgloss.Style
	helpKey    lipgloss.Style
	helpDesc   lipgloss.Style
	helpSep    lipgloss.Style
	plain      lipgloss.Style                      // text left in the terminal's colours
	tokens     map[domain.TokenKind]lipgloss.Style // the parts of a task line, priorities aside
	priorities map[string]lipgloss.Style           // priorities, by token text: "(A)"
}

func newStyles(t Theme) styles {
	fg := func(c lipgloss.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }
	return styles{
		theme:    t,
		task:     lipgloss.NewStyle().Foreground(t.Foreground).Background(t.Selection),
		status:   lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Padding(0, 1),
		helpKey:  lipgloss.NewStyle().Foreground(t.Muted).Bold(true),
		helpDesc: lipgloss.NewStyle().Foreground(t.Help),
		helpSep:  lipgloss.NewStyle().Foreground(t.Separator),
		plain:    lipgloss.NewStyle(),
		tokens: map[domain.TokenKind]lipgloss.Style{
			domain.TokenDate:       fg(t.Date),
			domain.TokenPlusTag:    fg(t.PlusTag),
			domain.TokenAtTag:      fg(t.AtTag),
			domain.TokenHashTag:    fg(t.HashTag),
			domain.TokenAnnotation: fg(t.Annotation),
		},
		priorities: map[string]lipgloss.Style{
			"(A)": fg(t.PriorityA),
			"(B)": fg(t.PriorityB),
			"(C)": fg(t.PriorityC),
		},
	}
}

// token is the style of one word of a task line on an unselected row.
// Plain words, the completion mark and priorities other than A to C
// keep the terminal's colours.
func (s styles) token(kind domain.TokenKind, word string) lipgloss.Style {
	if kind == domain.TokenPriority {
		if st, ok := s.priorities[word]; ok {
			return st
		}
		return s.plain
	}
	if st, ok := s.tokens[kind]; ok {
		return st
	}
	return s.plain
}

// selected styles a tab as the current one.
func (s styles) selected(l lipgloss.Style) lipgloss.Style {
	return l.
		Foreground(s.theme.Foreground).
		Background(s.theme.Selection).
		Bold(true)
}
