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

// Parser will parse todo entries
type Parser struct {
	ast        *parsec.AST
	rootParser parsec.Parser
}

func NewParser(ast *parsec.AST, root parsec.Parser) *Parser {
	return &Parser{
		ast:        ast,
		rootParser: root,
	}
}

// Parse will parse a todo entry into a queryable interface
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

func (p *Parser) Query(q string) []parsec.Queryable {
	ch := make(chan parsec.Queryable, 100)
	p.ast.Query(q, ch)
	nodes := []parsec.Queryable{}
	for node := range ch {
		nodes = append(nodes, node)
	}
	return nodes
}

func (p *Parser) QueryOne(q string) parsec.Queryable {
	ch := make(chan parsec.Queryable, 100)
	p.ast.Query(q, ch)

	for node := range ch {
		return node
	}
	return nil
}

func NodeToValue(n parsec.Queryable) string {
	return n.GetValue()
}

func MapNodes(nodes []parsec.Queryable, fn func(parsec.Queryable) string) []string {
	vs := []string{}
	for _, n := range nodes {
		vs = append(vs, fn(n))
	}
	return vs
}

func Filter(nodes []parsec.Queryable, fn func(parsec.Queryable) bool) []parsec.Queryable {
func Select(nodes []parsec.Queryable, fn func(parsec.Queryable) bool) []parsec.Queryable {
	ns := []parsec.Queryable{}
	for _, n := range nodes {
		if fn(n) {
			ns = append(ns, n)
		}
	}
	return ns
}
