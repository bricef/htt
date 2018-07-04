package todo

import (
	"fmt"
	"io"
	"os"

	parsec "github.com/prataprc/goparsec"
)

type TodoParser struct {
	ast        *parsec.AST
	rootParser parsec.Parser
}

func (parser TodoParser) Parse(text string) parsec.Queryable {
	scanner := parsec.NewScanner([]byte(text))
	rootQueriable, _ := parser.ast.Parsewith(parser.rootParser, scanner)
	//parser.ast.Reset()
	return rootQueriable
}

func prettyprint(w io.Writer, prefix string, node parsec.Queryable) {
	if node.IsTerminal() {
		fmt.Fprintf(w, "%v*%v: %q\n", prefix, node.GetName(), node.GetValue())
		return
	}
	fmt.Fprintf(w, "%v%v @ %v\n", prefix, node.GetName(), node.GetPosition())
	for _, child := range node.GetChildren() {
		prettyprint(w, prefix+"  ", child)
	}
}

func (parser TodoParser) Prettyprint(node parsec.Queryable) {
	prettyprint(os.Stdout, "", node)
}

func NewTodoParser() *TodoParser {
	ast := parsec.NewAST("TODO", 1000)

	tag := ast.OrdChoice("TAG", nil,
		parsec.Token(`[+][[:graph:]]+`, "PLUSTAG"),
		parsec.Token(`[@][[:graph:]]+`, "ATTAG"),
		parsec.Token(`[#][[:graph:]]+`, "HASHTAG"),
	)

	kvPair := ast.And("KVPAIR", nil,
		// we use this long regexp instead of :graph: to omit ':' There's probably a regexpy
		// way of doing this better
		parsec.Token(`[^#@+:][A-Za-z0-9!"#$%&'()*+,\-./;<=>?@[\\\]^_{|}~]*`, "KEY"),
		parsec.TokenExact(":", "COLON"),
		parsec.TokenExact(`[A-Za-z0-9!"#$%&'()*+,\-./;<=>?@[\\\]^_{|}~]+`, "VALUE"),
	)

	word := ast.OrdChoice("WORD", nil,
		parsec.Token(`[^@#+][[:graph:]]*`, "WORD"),
		parsec.Token(`[@#+]$`, "WORD"),
	)

	token := ast.OrdChoice("TOKEN", nil, kvPair, word, tag)

	createdDate := parsec.Token(`[0-9]{4}-[0-9]{2}-[0-9]{2}`, "CREATIONDATE")
	completeDate := parsec.Token(`[0-9]{4}-[0-9]{2}-[0-9]{2}`, "COMPLETIONDATE")

	priority := parsec.Token(`\([A-Z]\)[[:space:]]+`, "PRIORITY")

	completeMark := parsec.Token(`x[[:space:]]+`, "COMPLETED")

	TODO := ast.And("TODO", nil,
		ast.OrdChoice("PREAMBLE", nil,
			ast.And("PREAMBLE", nil,
				completeMark,
				ast.Maybe("PRIORITY", nil, priority),
				ast.Maybe("COMPLETEDAT", nil, completeDate),
				ast.Maybe("CREATEDAT", nil, createdDate),
			),
			ast.And("PREAMBLE", nil,
				ast.Maybe("PRIORITY", nil, priority),
				ast.Maybe("CREATEDAT", nil, createdDate),
			),
		),
		ast.Many("WORDS", nil, token),
	)
	return &TodoParser{
		ast:        ast,
		rootParser: TODO,
	}
}
