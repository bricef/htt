package domain

import (
	"slices"
	"strings"

	parsec "github.com/prataprc/goparsec"
)

// TokenKind is what the parser reads a word of a todo.txt line as.
type TokenKind int

const (
	TokenWord       TokenKind = iota // part of the task's text
	TokenDone                        // the completion mark, x
	TokenPriority                    // (A)
	TokenDate                        // the completed-on or created-on date
	TokenPlusTag                     // +tag
	TokenAtTag                       // @tag
	TokenHashTag                     // #tag
	TokenAnnotation                  // key:value
)

// Token is one word of a line and what it is. Start and End are byte
// offsets: the word is line[Start:End].
type Token struct {
	Kind       TokenKind
	Start, End int
}

// tokenKinds maps the grammar's node names (parser.go) to token kinds.
// Every word of a line is exactly one of these nodes.
var tokenKinds = map[string]TokenKind{
	"COMPLETED":   TokenDone,
	"PRIORITY":    TokenPriority,
	"COMPLETEDAT": TokenDate,
	"CREATEDAT":   TokenDate,
	"WORD":        TokenWord,
	"PLUSTAG":     TokenPlusTag,
	"ATTAG":       TokenAtTag,
	"HASHTAG":     TokenHashTag,
	"KVPAIR":      TokenAnnotation,
}

// Tokens splits a todo.txt line into its words, in order, each with
// what the parser reads it as: the same parse NewTask makes, so a
// display that colours tokens shows what htt understood. Every word of
// the line is one token; what lies between tokens is whitespace.
func Tokens(line string) []Token {
	parser := NewTodoParser()
	root, complete := parser.Parse(line)
	if !complete {
		// Unreachable while the grammar reads every line (see
		// NewTask); plain words are the safe fallback for a display.
		return plainWords(line)
	}
	var out []Token
	var walk func(n parsec.Queryable)
	walk = func(n parsec.Queryable) {
		if kind, ok := tokenKinds[n.GetName()]; ok {
			start := n.GetPosition()
			out = append(out, Token{Kind: kind, Start: start, End: wordEnd(line, start)})
			return
		}
		for _, child := range n.GetChildren() {
			walk(child)
		}
	}
	walk(root)
	slices.SortFunc(out, func(a, b Token) int { return a.Start - b.Start })
	return out
}

// Tokens splits the task's line into its words; see Tokens.
func (t *Task) Tokens() []Token { return Tokens(t.Raw) }

// wordEnd is the offset just past the word starting at start.
func wordEnd(line string, start int) int {
	if i := strings.IndexAny(line[start:], wordSpace); i >= 0 {
		return start + i
	}
	return len(line)
}

// plainWords labels every word of line a TokenWord.
func plainWords(line string) []Token {
	var out []Token
	for start := 0; start < len(line); {
		if strings.ContainsRune(wordSpace, rune(line[start])) {
			start++
			continue
		}
		end := wordEnd(line, start)
		out = append(out, Token{Kind: TokenWord, Start: start, End: end})
		start = end
	}
	return out
}
