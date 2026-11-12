package domain

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/bricef/htt/internal/parseutils"
	parsec "github.com/prataprc/goparsec"
)

// customPriority keeps the priority's letter. Like datePromote, it keeps
// the token's own position (s is the scanner after it), which Tokens
// reports.
func customPriority(name string, s parsec.Scanner, node parsec.Queryable) parsec.Queryable {
	t := parsec.NewTerminal(name, string(node.GetValue()[1]), node.GetPosition())
	t.SetAttribute("class", "term")
	return t
}

// datePromote rejects nodes that don't parse as YYYY-MM-DD, and on a
// successful match returns a fresh Terminal carrying the Maybe's own
// name (COMPLETEDAT / CREATEDAT). The previous implementation returned
// the inner node unchanged, leaving it under the Token's name
// (COMPLETIONDATE / CREATIONDATE), so QueryOne("COMPLETEDAT") /
// QueryOne("CREATEDAT") in task.go silently found nothing and the
// CompletedOn / CreatedOn fields stayed zero.
//
// Returning nil on a non-date signals no-match so the parser can try
// the alternative production — that's the right behavior for an
// ambiguous grammar, so no logging is needed here.
//
// The date token carries the whitespace that ends it (see dateToken);
// the value doesn't.
func datePromote(name string, s parsec.Scanner, node parsec.Queryable) parsec.Queryable {
	value := strings.TrimRight(node.GetValue(), wordSpace)
	if _, err := time.ParseInLocation("2006-01-02", value, time.Local); err != nil {
		return nil
	}
	t := parsec.NewTerminal(name, value, node.GetPosition())
	t.SetAttribute("class", "term")
	return t
}

func KVPAIR2String(node parsec.Queryable) (string, string, error) {
	if node.GetName() != "KVPAIR" { // annoying we can't use the type system for this.
		return "", "", errors.New("tried to parse something other than a KVPAIR")
	}
	key := node.GetChildren()[0].GetValue()
	value := node.GetChildren()[2].GetValue()
	return key, value, nil
}

// A line is a sequence of words separated by whitespace: the characters
// the scanner skips between tokens. Every other character, non-ASCII
// letters included, belongs to a word, and every token below is a whole
// word: none stops part-way through one, and every word matches one of
// them. Rewriting a task rebuilds its line from the parsed parts, so a
// token that split or skipped a word would lose text from the file.
const wordSpace = " \t\r\n"

// nonSpace matches one character of a word.
const nonSpace = `[^ \t\r\n]`

// kvpairKeyRegexp is the key of a key:value annotation: no colon, and
// not starting with a tag sigil.
var kvpairKeyRegexp = `[^#@+: \t\r\n][^: \t\r\n]*`

// Note that unlike the standard, we allow ':' in the value.
// This is so that ISO timestamps can be used
var kvpairValueRegexp = nonSpace + `+`

// linkSchemes are the common URI schemes whose links have no "//"
// after the colon. Links with one (https://, ssh://, file:///) are
// recognised by it, whatever the scheme.
var linkSchemes = []string{"mailto", "tel", "sms", "magnet"}

var allDigits = regexp.MustCompile(`^[0-9]+$`)

// isAnnotation reports whether key:value is an annotation rather than
// a word that happens to contain a colon. Times and ratios (10:30,
// 12:30pm, 3:16: the key is all digits) and links (https://…,
// mailto:…) are part of what the task says, so they stay words, out of
// Task.Annotations.
func isAnnotation(key, value string) bool {
	isTime := allDigits.MatchString(key)
	isLink := strings.HasPrefix(value, "//") || slices.Contains(linkSchemes, strings.ToLower(key))
	return !isTime && !isLink
}

// annotationOnly keeps a parsed key:value that isAnnotation accepts.
// Returning nil rejects the match, and the parser reads the word as a
// plain one instead.
func annotationOnly(name string, s parsec.Scanner, node parsec.Queryable) parsec.Queryable {
	if kv := node.GetChildren(); len(kv) == 3 && !isAnnotation(kv[0].GetValue(), kv[2].GetValue()) {
		return nil
	}
	return node
}

// dateToken matches a YYYY-MM-DD word. It takes the whitespace after
// the date, since a regexp can't look ahead; datePromote trims it.
const dateToken = `[0-9]{4}-[0-9]{2}-[0-9]{2}(?:[ \t\r\n]|$)`

//NewTodoParser creates a new Parser
func NewTodoParser() *parseutils.Parser {
	g := parsec.NewAST("TODO", 1000)

	tag := g.OrdChoice("TAG", nil,
		parsec.Token(`[+]`+nonSpace+`+`, "PLUSTAG"),
		parsec.Token(`[@]`+nonSpace+`+`, "ATTAG"),
		parsec.Token(`[#]`+nonSpace+`+`, "HASHTAG"),
	)

	kvPair := g.And("KVPAIR", annotationOnly,
		parsec.Token(kvpairKeyRegexp, "KEY"),
		parsec.TokenExact(":", "COLON"),
		parsec.TokenExact(kvpairValueRegexp, "VALUE"),
	)

	word := parsec.Token(`[^@#+ \t\r\n]`+nonSpace+`*`, "WORD")
	// A sigil on its own ("buy + milk") is a word, not a tag. Tried
	// after tag, so it only matches a sigil followed by whitespace.
	loneSigil := parsec.Token(`[@#+]`, "WORD")

	token := g.OrdChoice("TOKEN", nil, kvPair, word, tag, loneSigil)

	createdDate := parsec.Token(dateToken, "CREATIONDATE")
	completeDate := parsec.Token(dateToken, "COMPLETIONDATE")

	priority := parsec.Token(`\([A-Z]\)[ \t\r\n]+`, "PRIORITY")

	completeMark := parsec.Token(`x[ \t\r\n]+`, "COMPLETED")

	// The first branch is the "completed task" shape: `x` mark
	// required, then optional priority, completed-on, created-on.
	// The second branch is the "open task" shape: optional priority
	// then optional created-on.
	//
	// Making the `x` mark required in the first branch (rather than
	// Maybe) is what lets the OrdChoice correctly route a bare-date
	// line like "2026-05-10 hello" into the second branch — where
	// the date is then tagged CREATEDAT, not COMPLETEDAT. With the
	// previous Maybe, the first branch matched without `x` and
	// greedy-grabbed any leading date as COMPLETEDAT, leaving
	// CreatedOn perpetually zero.
	TODO := g.And("TODO", nil,
		g.OrdChoice("PREAMBLE", nil,
			g.And("PREAMBLE", nil,
				completeMark,
				g.Maybe("PRIORITY", customPriority, priority),
				g.Maybe("COMPLETEDAT", datePromote, completeDate),
				g.Maybe("CREATEDAT", datePromote, createdDate),
			),
			g.And("PREAMBLE", nil,
				g.Maybe("PRIORITY", customPriority, priority),
				g.Maybe("CREATEDAT", datePromote, createdDate),
			),
		),
		// Many fails on zero words, and a line can be all preamble
		// ("x 2026-05-20").
		g.Maybe("BODY", nil, g.Many("WORDS", nil, token)),
	)
	return parseutils.NewParser(g, TODO)
}
