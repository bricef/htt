package parseutils

import (
	"fmt"
	"io"
	"os"

	parsec "github.com/prataprc/goparsec"
)

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

// Parser utility object
type Parser struct {
	ast        *parsec.AST
	rootParser parsec.Parser
}

// NewParser Creates a new Parser convenience object from goparsec primitives
func NewParser(ast *parsec.AST, root parsec.Parser) *Parser {
	return &Parser{
		ast:        ast,
		rootParser: root,
	}
}

// Parse a string
// FUTURE: Will return a parsed todo.Tasks
func (p *Parser) Parse(text string) parsec.Queryable {
	scanner := parsec.NewScanner([]byte(text))
	rootQueriable, _ := p.ast.Parsewith(p.rootParser, scanner)
	return rootQueriable
}

// Reset will reset the parser.
func (p *Parser) Reset() {
	p.ast.Reset()
}

// Prettyprint the parsed AST
func (p *Parser) Prettyprint(node parsec.Queryable) {
	prettyprint(os.Stdout, "", node)
}

// Query the AST using goparsec's query language
// See https://prataprc.github.io/astquery.io/ for language spec.
func (p *Parser) Query(q string) []parsec.Queryable {
	ch := make(chan parsec.Queryable, 100)
	p.ast.Query(q, ch)
	nodes := []parsec.Queryable{}
	for node := range ch {
		nodes = append(nodes, node)
	}
	return nodes
}

// QueryOne queries the AST using goparsec's query language
// QueryOne will only return the first matching item, even if there are more.
// See https://prataprc.github.io/astquery.io/ for language spec.
func (p *Parser) QueryOne(q string) parsec.Queryable {
	ch := make(chan parsec.Queryable, 100)
	p.ast.Query(q, ch)

	for node := range ch {
		return node
	}
	return nil
}

// NodeToValue is a utility function to be used in higher order functions
func NodeToValue(n parsec.Queryable) string {
	return n.GetValue()
}

// MapNodes will map the nodes using the function fn
func MapNodes(nodes []parsec.Queryable, fn func(parsec.Queryable) string) []string {
	vs := []string{}
	for _, n := range nodes {
		vs = append(vs, fn(n))
	}
	return vs
}

// Select will filter out the nodes based on a predicate.
// Items matching the predicate will be preserved.
func Select(nodes []parsec.Queryable, fn func(parsec.Queryable) bool) []parsec.Queryable {
	ns := []parsec.Queryable{}
	for _, n := range nodes {
		if fn(n) {
			ns = append(ns, n)
		}
	}
	return ns
}
// Godammit, I miss generics.

// Transform will apply a parsec.Queryable -> parsec.Queryable mapper to a tree, dealing with
// setting the children properly and returning the root node. This can be used to transform an
// AST or for side effects.
func Transform(node parsec.Queryable, fn func(node parsec.Queryable) parsec.Queryable) parsec.Queryable {
	newNode := fn(node)
	for i n := range node.GetChildren() {
		newNode.
	}
}
