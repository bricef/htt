package cli

import (
	"strings"
	"unicode"
)

// replWords splits a REPL line into the words a command gets, reading
// quotes as the shell does, so a line typed in the REPL means what it
// means in a terminal: `/log end --at "yesterday 17:30"` gives --at
// one word, `yesterday 17:30`.
//
//   - Words are separated by whitespace, as strings.Fields splits them;
//     a line without quotes splits exactly as strings.Fields does.
//   - A " or ' that starts a word, or follows a = in one (as in
//     --at="yesterday 17:30"), opens a quoted part, which runs to the
//     next quote of the same kind that ends a word. The quotes are
//     dropped and everything between them kept, spaces included.
//   - Any other quote is kept as typed: an apostrophe inside a word
//     (don't, Sam's), or a quote that nothing closes. The shell would
//     wait for its closing quote; the REPL reads it as text.
//
// It isn't a shell: backslashes, $ and * are text.
func replWords(line string) []string {
	var words []string
	var word strings.Builder
	inWord := false
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case unicode.IsSpace(r):
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
			continue
		case (r == '"' || r == '\'') && (!inWord || runes[i-1] == '='):
			if end := closingQuote(runes, i); end >= 0 {
				word.WriteString(string(runes[i+1 : end]))
				inWord = true
				i = end
				continue
			}
		}
		word.WriteRune(r)
		inWord = true
	}
	if inWord {
		words = append(words, word.String())
	}
	return words
}

// closingQuote is the index of the quote that closes the one at open:
// the next quote of its kind that ends a word, or -1 if there's none.
func closingQuote(runes []rune, open int) int {
	for j := open + 1; j < len(runes); j++ {
		if runes[j] == runes[open] && (j+1 == len(runes) || unicode.IsSpace(runes[j+1])) {
			return j
		}
	}
	return -1
}
