package domain

import (
	"errors"
	"time"

	"github.com/bricef/htt/internal/parseutils"
	parsec "github.com/prataprc/goparsec"
)

func customPriority(name string, s parsec.Scanner, node parsec.Queryable) parsec.Queryable {
	t := parsec.NewTerminal(name, string(node.GetValue()[1]), s.GetCursor())
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
func datePromote(name string, s parsec.Scanner, node parsec.Queryable) parsec.Queryable {
	value := node.GetValue()
	if _, err := time.ParseInLocation("2006-01-02", value, time.Local); err != nil {
		return nil
	}
	t := parsec.NewTerminal(name, value, s.GetCursor())
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

var kvpairKeyRegexp = `[^#@+:][A-Za-z0-9!"#$%&'()*+,\-./;<=>?@[\\\]^_{|}~]*`

// Note that unlike the standard, we allow ':' in the value.
// This is so that ISO timestamps can be used
var kvpairValueRegexp = `[A-Za-z0-9!"#$%&'()*+,\-./;<=>?@[\\\]^_{|}~:]+`

//NewTodoParser creates a new Parser
func NewTodoParser() *parseutils.Parser {
	g := parsec.NewAST("TODO", 1000)

	tag := g.OrdChoice("TAG", nil,
		parsec.Token(`[+][[:graph:]]+`, "PLUSTAG"),
		parsec.Token(`[@][[:graph:]]+`, "ATTAG"),
		parsec.Token(`[#][[:graph:]]+`, "HASHTAG"),
	)

	kvPair := g.And("KVPAIR", nil,
		// we use this long regexp instead of :graph: to omit ':' There's probably a regexpy
		// way of doing this better
		parsec.Token(kvpairKeyRegexp, "KEY"),
		parsec.TokenExact(":", "COLON"),
		parsec.TokenExact(kvpairValueRegexp, "VALUE"),
	)

	word := g.OrdChoice("WORD", nil,
		parsec.Token(`[^@#+][[:graph:]]*`, "WORD"),
		parsec.Token(`[@#+]$`, "WORD"),
	)

	token := g.OrdChoice("TOKEN", nil, kvPair, word, tag)

	createdDate := parsec.Token(`[0-9]{4}-[0-9]{2}-[0-9]{2}`, "CREATIONDATE")
	completeDate := parsec.Token(`[0-9]{4}-[0-9]{2}-[0-9]{2}`, "COMPLETIONDATE")

	priority := parsec.Token(`\([A-Z]\)[[:space:]]+`, "PRIORITY")

	completeMark := parsec.Token(`x[[:space:]]+`, "COMPLETED")

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
		g.Many("WORDS", nil, token),
	)
	return parseutils.NewParser(g, TODO)
}
