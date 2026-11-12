package parseutils

import (
	"strings"
	"testing"
	"time"

	parsec "github.com/prataprc/goparsec"
)

// within fails the test if f hasn't returned after a few seconds, so a
// hang shows up as a failure rather than a stuck test run.
func within(t *testing.T, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("did not return within 5s")
	}
}

func TestQuery_ReturnsEveryMatchHoweverMany(t *testing.T) {
	// Before: 100 matches filled the channel and the 101st blocked.
	g := parsec.NewAST("AS", 100)
	p := NewParser(g, g.Many("AS", nil, parsec.Token(`a`, "A")))
	if _, complete := p.Parse(strings.Repeat("a ", 250)); !complete {
		t.Fatal("parse incomplete")
	}
	within(t, func() {
		if got := len(p.Query("A")); got != 250 {
			t.Errorf("Query found %d matches, want 250", got)
		}
		if p.QueryOne("A") == nil || p.QueryOne("B") != nil {
			t.Errorf("QueryOne should find the first A and no B")
		}
	})
}

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
