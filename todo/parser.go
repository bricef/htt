package todo

import (
	"errors"
	"time"

	"github.com/hypotheticalco/tracker-client/parseutils"
	"github.com/hypotheticalco/tracker-client/utils"
	parsec "github.com/prataprc/goparsec"
)

func customPriority(name string, s parsec.Scanner, node parsec.Queryable) parsec.Queryable {
	t := parsec.NewTerminal(name, string(node.GetValue()[1]), s.GetCursor())
	t.SetAttribute("class", "term")
	return t
}

func customCompleted(name string, s parsec.Scanner, node parsec.Queryable) parsec.Queryable {
	t := parsec.NewTerminal(name, "x", s.GetCursor())
	t.SetAttribute("class", "term")
	return t
}

func dateValidation(name string, s parsec.Scanner, node parsec.Queryable) parsec.Queryable {
	_, err := time.Parse("2006-01-02", node.GetValue())
	if err != nil { // Not a valid date in the format specified
		utils.Failure("Could not parse give date: ", err)
		return nil
	}
	return node
}

func KVPAIR2String(node parsec.Queryable) (string, string, error) {
	if node.GetName() != "KVPAIR" { // annoying we can't use the type system for this.
		return "", "", errors.New("Tried to parse something other than a KVPAIR")
	}
	key := node.GetChildren()[0].GetValue()
	value := node.GetChildren()[2].GetValue()
	return key, value, nil
}

var kvpairKeyRegexp = `[^#@+:][A-Za-z0-9!"#$%&'()*+,\-./;<=>?@[\\\]^_{|}~]*`
var kvpairValueRegexp = `[A-Za-z0-9!"#$%&'()*+,\-./;<=>?@[\\\]^_{|}~]+`

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

	TODO := g.And("TODO", nil,
		g.OrdChoice("PREAMBLE", nil,
			g.And("PREAMBLE", nil,
				g.Maybe("COMPLETED", customCompleted, completeMark),
				g.Maybe("PRIORITY", customPriority, priority),
				g.Maybe("COMPLETEDAT", dateValidation, completeDate),
				g.Maybe("CREATEDAT", dateValidation, createdDate),
			),
			g.And("PREAMBLE", nil,
				g.Maybe("PRIORITY", customPriority, priority),
				g.Maybe("CREATEDAT", nil, createdDate),
			),
		),
		g.Many("WORDS", nil, token),
	)
	return parseutils.NewParser(g, TODO)
}
