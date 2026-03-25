package config

import (
	"fmt"
	"strings"
	"unicode"
)

type RunExpr interface {
	isRunExpr()
}

type RunRef struct {
	Name string
}

func (RunRef) isRunExpr() {}

type RunSeq struct {
	Nodes []RunExpr
}

func (RunSeq) isRunExpr() {}

type RunPar struct {
	Nodes []RunExpr
}

func (RunPar) isRunExpr() {}

func ParseRunExpr(input string) (RunExpr, error) {
	p := &runParser{input: input}
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.pos < len(p.input) {
		return nil, fmt.Errorf("unexpected token near %q", p.input[p.pos:])
	}
	return expr, nil
}

func RunExprRefs(expr RunExpr) []string {
	seen := map[string]bool{}
	var refs []string
	var visit func(RunExpr)
	visit = func(node RunExpr) {
		switch n := node.(type) {
		case RunRef:
			if !seen[n.Name] {
				seen[n.Name] = true
				refs = append(refs, n.Name)
			}
		case RunSeq:
			for _, child := range n.Nodes {
				visit(child)
			}
		case RunPar:
			for _, child := range n.Nodes {
				visit(child)
			}
		}
	}
	visit(expr)
	return refs
}

type runParser struct {
	input string
	pos   int
}

func (p *runParser) parseExpr() (RunExpr, error) {
	first, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	nodes := []RunExpr{first}
	for {
		p.skipSpace()
		if !p.consume("->") {
			break
		}
		next, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, next)
	}
	if len(nodes) == 1 {
		return nodes[0], nil
	}
	return RunSeq{Nodes: nodes}, nil
}

func (p *runParser) parseTerm() (RunExpr, error) {
	p.skipSpace()
	if p.consumeParKeyword() {
		p.skipSpace()
		if !p.consume("(") {
			return nil, fmt.Errorf("expected '(' after par")
		}
		nodes := []RunExpr{}
		for {
			p.skipSpace()
			if p.consume(")") {
				break
			}
			node, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
			p.skipSpace()
			if p.consume(")") {
				break
			}
			if !p.consume(",") {
				return nil, fmt.Errorf("expected ',' or ')' in par()")
			}
		}
		if len(nodes) == 0 {
			return nil, fmt.Errorf("par() requires at least one task")
		}
		return RunPar{Nodes: nodes}, nil
	}

	name := p.parseIdentifier()
	if name == "" {
		return nil, fmt.Errorf("expected task reference")
	}
	return RunRef{Name: name}, nil
}

func (p *runParser) consumeParKeyword() bool {
	p.skipSpace()
	if !strings.HasPrefix(p.input[p.pos:], "par") {
		return false
	}
	next := p.pos + len("par")
	if next < len(p.input) {
		r := rune(p.input[next])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._/-", r) {
			return false
		}
	}
	p.pos = next
	return true
}

func (p *runParser) parseIdentifier() string {
	p.skipSpace()
	start := p.pos
	for p.pos < len(p.input) {
		r := rune(p.input[p.pos])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._/-", r) {
			p.pos++
			continue
		}
		break
	}
	if start == p.pos {
		return ""
	}
	return p.input[start:p.pos]
}

func (p *runParser) skipSpace() {
	for p.pos < len(p.input) {
		if p.input[p.pos] != ' ' && p.input[p.pos] != '\t' && p.input[p.pos] != '\n' && p.input[p.pos] != '\r' {
			return
		}
		p.pos++
	}
}

func (p *runParser) consume(token string) bool {
	p.skipSpace()
	if strings.HasPrefix(p.input[p.pos:], token) {
		p.pos += len(token)
		return true
	}
	return false
}
