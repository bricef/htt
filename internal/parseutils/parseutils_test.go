package parseutils

import (
	"testing"

	parsec "github.com/prataprc/goparsec"
)

func TestParse_ReportsWhetherItReadEverything(t *testing.T) {
	g := parsec.NewAST("AS", 100)
	p := NewParser(g, g.Many("AS", nil, parsec.Token(`a`, "A")))

	cases := []struct {
		text     string
		complete bool
	}{
		{"a a a", true},
		{"a a  ", true}, // trailing whitespace is nothing left unread
		{"a a b", false},
		{"b", false}, // no tree at all
		{"", false},
	}
	for _, tc := range cases {
		if _, complete := p.Parse(tc.text); complete != tc.complete {
			t.Errorf("Parse(%q) complete = %v, want %v", tc.text, complete, tc.complete)
		}
	}
}
