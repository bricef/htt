package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/bricef/htt/internal/domain"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// cell is one character of rendered output with the attributes the
// terminal would draw it with.
type cell struct {
	r         rune
	fg, bg    string // "#rrggbb", or "" for the terminal's own
	underline bool
}

var sgr = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// cells interprets the SGR sequences in s (as lipgloss writes them), so
// tests can ask what colour each character is drawn in.
func cells(s string) []cell {
	var out []cell
	var cur cell
	for len(s) > 0 {
		if loc := sgr.FindStringSubmatchIndex(s); loc != nil && loc[0] == 0 {
			params := strings.Split(s[loc[2]:loc[3]], ";")
			for i := 0; i < len(params); i++ {
				switch params[i] {
				case "", "0":
					cur = cell{}
				case "4":
					cur.underline = true
				case "24":
					cur.underline = false
				case "38", "48":
					if i+4 < len(params) && params[i+1] == "2" {
						var rgb [3]int
						for j := range rgb {
							rgb[j], _ = strconv.Atoi(params[i+2+j])
						}
						hex := fmt.Sprintf("#%02x%02x%02x", rgb[0], rgb[1], rgb[2])
						if params[i] == "38" {
							cur.fg = hex
						} else {
							cur.bg = hex
						}
						i += 4
					}
				case "39":
					cur.fg = ""
				case "49":
					cur.bg = ""
				}
			}
			s = s[loc[1]:]
			continue
		}
		r, size := []rune(s[:min(len(s), 4)])[0], 0
		size = len(string(r))
		c := cur
		c.r = r
		out = append(out, c)
		s = s[size:]
	}
	return out
}

// sameColour compares #rrggbb colours, allowing lipgloss's off-by-one
// rounding in each channel.
func sameColour(a string, b lipgloss.Color) bool {
	if a == "" || b == "" {
		return a == string(b)
	}
	pa, _ := strconv.ParseUint(a[1:], 16, 32)
	pb, _ := strconv.ParseUint(string(b)[1:], 16, 32)
	for shift := 0; shift <= 16; shift += 8 {
		d := int(pa>>shift&0xff) - int(pb>>shift&0xff)
		if d < -1 || d > 1 {
			return false
		}
	}
	return true
}

// wordCells finds word in the cells' text and returns its cells.
func wordCells(t *testing.T, cs []cell, word string) []cell {
	t.Helper()
	var text []rune
	for _, c := range cs {
		text = append(text, c.r)
	}
	i := strings.Index(string(text), word)
	if i < 0 {
		t.Fatalf("%q not in %q", word, string(text))
	}
	start := len([]rune(string(text)[:i]))
	return cs[start : start+len([]rune(word))]
}

func TestLine_ColoursEachKindOfWord(t *testing.T) {
	trueColor(t)
	theme := themes[DefaultThemeName]
	st := newStyles(theme)
	line := "x (A) 2026-05-20 call +mum @phone #garden due:2026-05-25 now (B) (C)"
	cs := cells(st.line(line, domain.Tokens(line), nil, false))

	cases := []struct {
		word string
		fg   lipgloss.Color
	}{
		{"x ", ""}, {"(A)", theme.PriorityA}, {"2026-05-20", theme.Date}, {" call ", ""},
		{"+mum", theme.PlusTag}, {"@phone", theme.AtTag}, {"#garden", theme.HashTag},
		{"due:2026-05-25", theme.Annotation}, {" now (B)", ""}, {"(C)", ""},
	}
	for _, tc := range cases {
		for _, c := range wordCells(t, cs, tc.word) {
			if !sameColour(c.fg, tc.fg) || c.bg != "" || c.underline {
				t.Errorf("%q: %q drawn fg=%q bg=%q underline=%v, want fg=%q", tc.word, c.r, c.fg, c.bg, c.underline, tc.fg)
				break
			}
		}
	}
	// "(B)" and "(C)" above are words of the text, not priorities: only
	// the preamble's priority is coloured.
	cs = cells(st.line("(B) b", domain.Tokens("(B) b"), nil, false))
	if c := wordCells(t, cs, "(B)")[0]; !sameColour(c.fg, theme.PriorityB) {
		t.Errorf("(B) priority drawn %q, want %q", c.fg, theme.PriorityB)
	}
	cs = cells(st.line("(C) c", domain.Tokens("(C) c"), nil, false))
	if c := wordCells(t, cs, "(C)")[0]; !sameColour(c.fg, theme.PriorityC) {
		t.Errorf("(C) priority drawn %q, want %q", c.fg, theme.PriorityC)
	}
	cs = cells(st.line("(D) d", domain.Tokens("(D) d"), nil, false))
	if c := wordCells(t, cs, "(D)")[0]; c.fg != "" {
		t.Errorf("(D) has no colour role, drawn %q", c.fg)
	}
}

func TestLine_SelectedRowKeepsTheSelectionColours(t *testing.T) {
	trueColor(t)
	theme := themes[DefaultThemeName]
	st := newStyles(theme)
	line := "(A) call +mum due:2026-05-25"
	for _, c := range cells(st.line(line, domain.Tokens(line), nil, true)) {
		if !sameColour(c.fg, theme.Foreground) || !sameColour(c.bg, theme.Selection) {
			t.Fatalf("selected row: %q drawn fg=%q bg=%q, want foreground on selection", c.r, c.fg, c.bg)
		}
	}
}

func TestLine_UnderlinesSearchMatchesByByteOffset(t *testing.T) {
	trueColor(t)
	st := newStyles(themes[DefaultThemeName])
	// The fuzzy matcher reports byte offsets. "crème" puts a two-byte
	// character before the match, so treating them as rune offsets
	// would underline the wrong characters.
	line := "crème brûlée +dessert"
	start := strings.Index(line, "brûlée")
	var matched []int
	for i := range line[start : start+len("brûlée")] {
		matched = append(matched, start+i)
	}
	for _, selected := range []bool{false, true} {
		var underlined []rune
		for _, c := range cells(st.line(line, domain.Tokens(line), matched, selected)) {
			if c.underline {
				underlined = append(underlined, c.r)
			}
		}
		if string(underlined) != "brûlée" {
			t.Errorf("selected=%v: underlined %q, want brûlée", selected, string(underlined))
		}
	}
}

func TestLine_DrawsWhitespaceAsSpaces(t *testing.T) {
	st := newStyles(themes[DefaultThemeName])
	line := "a\tb  c\r"
	got := ansi.Strip(st.line(line, domain.Tokens(line), nil, false))
	if got != "a b  c " {
		t.Errorf("line drawn as %q, want %q", got, "a b  c ")
	}
}

func TestView_ColoursRowsAndUnderlinesSearchMatches(t *testing.T) {
	trueColor(t)
	theme := themes[DefaultThemeName]
	m, _ := seedSearchModel(t, "first", "buy milk +shop @town", "milk the goat @farm")
	m = search(t, m, "milk")
	// The search shows tasks 1 and 2; the cursor is on task 1. Task 2's
	// row is unselected, so it has token colours.
	cs := cells(m.View())

	shop := wordCells(t, cs, "+shop")
	if !sameColour(shop[0].fg, theme.Foreground) || !sameColour(shop[0].bg, theme.Selection) {
		t.Errorf("+shop on the selected row drawn fg=%q bg=%q, want the selection colours", shop[0].fg, shop[0].bg)
	}
	goat := wordCells(t, cs, "milk the goat")
	for i, c := range goat {
		if want := i < 4; c.underline != want {
			t.Errorf("unselected row: %q underline=%v, want %v (search matches only)", c.r, c.underline, want)
		}
		if c.bg != "" {
			t.Errorf("unselected row: %q has a background %q", c.r, c.bg)
		}
	}
	for _, c := range wordCells(t, cs, "@farm") {
		if !sameColour(c.fg, theme.AtTag) {
			t.Errorf("unselected row: @farm drawn %q, want the at_tag colour %q", c.fg, theme.AtTag)
		}
	}
}

func TestView_LongRowsEndInAnEllipsis(t *testing.T) {
	m, _ := seedSearchModel(t, "short", strings.Repeat("word ", 40)+"end")
	m.width = 40
	m.layout()
	view := m.View()
	for _, line := range strings.Split(view, "\n") {
		if w := ansi.StringWidth(line); w > m.width {
			t.Errorf("line is %d wide, more than the %d-wide screen: %q", w, m.width, ansi.Strip(line))
		}
	}
	if !regexp.MustCompile(`(?m)^ +1 word word .*…$`).MatchString(ansi.Strip(view)) {
		t.Errorf("the long row should be cut to the screen, ending in an ellipsis:\n%s", ansi.Strip(view))
	}
}
