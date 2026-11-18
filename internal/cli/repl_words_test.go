package cli

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

func TestReplWords(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"spaces only", " \t ", nil},
		{"plain words", "add buy milk", []string{"add", "buy", "milk"}},
		{"runs of spaces and tabs", "  add\tbuy   milk  ", []string{"add", "buy", "milk"}},

		// Quotes at the start of a word group words, as in the shell.
		{"double quotes", `/log end --at "yesterday 17:30"`, []string{"/log", "end", "--at", "yesterday 17:30"}},
		{"single quotes", `--at 'yesterday 17:30'`, []string{"--at", "yesterday 17:30"}},
		{"after =", `--at="2026-11-17 17:30"`, []string{"--at=2026-11-17 17:30"}},
		{"single quotes after =", `--at='yesterday 9:00' x`, []string{"--at=yesterday 9:00", "x"}},
		{"spaces kept inside", `"a  b"`, []string{"a  b"}},
		{"quoted word mid-line", `add "buy milk" today`, []string{"add", "buy milk", "today"}},
		{"empty quotes are an empty word", `--at ""`, []string{"--at", ""}},
		{"the other quote inside", `add "call Sam's dentist" now`, []string{"add", "call Sam's dentist", "now"}},
		{"double inside single", `'say "hi" now'`, []string{`say "hi" now`}},
		{"two quoted words", `"a b" "c d"`, []string{"a b", "c d"}},

		// Apostrophes and other mid-word quotes stay as typed.
		{"apostrophe", "add call Sam's dentist", []string{"add", "call", "Sam's", "dentist"}},
		{"two apostrophes", "don't forget Jo's keys", []string{"don't", "forget", "Jo's", "keys"}},
		{"apostrophe inside a single-quoted part", `'don't panic' now`, []string{"don't panic", "now"}},
		// The kids' apostrophe ends a word, but Sam's never opened a
		// quoted part for it to close.
		{"plural possessive", "fix Sam's and the kids' bikes", []string{"fix", "Sam's", "and", "the", "kids'", "bikes"}},
		{"mid-word double quote", `5" screen`, []string{`5"`, "screen"}},

		// A quote with nothing to close it stays as typed.
		{"unclosed double quote", `add "buy milk`, []string{"add", `"buy`, "milk"}},
		{"unclosed single quote", "'tis the season", []string{"'tis", "the", "season"}},
		{"lone quote", `a " b`, []string{"a", `"`, "b"}},
		// A quote only closes where a word ends.
		{"quote closes only at a word end", `"a"b c`, []string{`"a"b`, "c"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := replWords(c.in); !slices.Equal(got, c.want) {
				t.Errorf("replWords(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// A line without quotes splits exactly as before the REPL read
// quotes: at its whitespace (strings.Fields).
func TestReplWords_LinesWithoutQuotesSplitAsBefore(t *testing.T) {
	alphabet := []rune("ab=-+:@#/\\0 \t 　é漢")
	rng := rand.New(rand.NewPCG(1, 2))
	for range 5000 {
		line := randomLine(rng, alphabet, 30)
		if got, want := replWords(line), strings.Fields(line); !slices.Equal(got, want) {
			t.Fatalf("replWords(%q) = %q, want strings.Fields' %q", line, got, want)
		}
	}
}

// Wrapping each word in double quotes gives the words back, whatever
// spaces and apostrophes they hold, as the shell does.
func TestReplWords_QuotedWordsRoundTrip(t *testing.T) {
	alphabet := []rune("ab'=- \t:é")
	rng := rand.New(rand.NewPCG(3, 4))
	for range 5000 {
		words := make([]string, 1+rng.IntN(4))
		quoted := make([]string, len(words))
		for i := range words {
			words[i] = randomLine(rng, alphabet, 8)
			quoted[i] = `"` + words[i] + `"`
		}
		line := strings.Join(quoted, " ")
		if got := replWords(line); !slices.Equal(got, words) {
			t.Fatalf("replWords(%q) = %q, want %q", line, got, words)
		}
	}
}

func randomLine(rng *rand.Rand, alphabet []rune, maxLen int) string {
	var b strings.Builder
	for range rng.IntN(maxLen + 1) {
		b.WriteRune(alphabet[rng.IntN(len(alphabet))])
	}
	return b.String()
}
