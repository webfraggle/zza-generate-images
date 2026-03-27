package renderer

import (
	"fmt"
	"strconv"
	"unicode"
)

// evalIntExpr evaluates a simple integer arithmetic expression.
// Supported: integer literals, variable names (from vars), +, -, *, /, parentheses.
// Division is integer division; division by zero returns an error.
// Variable names may contain letters, digits, underscore, and dot (e.g. "loop.index").
func evalIntExpr(expr string, vars map[string]int) (int, error) {
	p := &exprParser{input: expr, vars: vars}
	val, err := p.parseExpr()
	if err != nil {
		return 0, fmt.Errorf("evalIntExpr %q: %w", expr, err)
	}
	p.skipSpace()
	if p.pos < len(p.input) {
		return 0, fmt.Errorf("evalIntExpr %q: unexpected character %q at position %d", expr, p.input[p.pos], p.pos)
	}
	return val, nil
}

const maxExprDepth = 50 // prevents stack exhaustion from deeply nested unary minus or parentheses

type exprParser struct {
	input string
	pos   int
	depth int
	vars  map[string]int
}

func (p *exprParser) skipSpace() {
	for p.pos < len(p.input) && p.input[p.pos] == ' ' {
		p.pos++
	}
}

// parseExpr handles + and -.
func (p *exprParser) parseExpr() (int, error) {
	val, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.input) {
			break
		}
		op := p.input[p.pos]
		if op != '+' && op != '-' {
			break
		}
		p.pos++
		p.skipSpace()
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			val += right
		} else {
			val -= right
		}
	}
	return val, nil
}

// parseTerm handles * and /.
func (p *exprParser) parseTerm() (int, error) {
	val, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.input) {
			break
		}
		op := p.input[p.pos]
		if op != '*' && op != '/' {
			break
		}
		p.pos++
		p.skipSpace()
		right, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		if op == '*' {
			val *= right
		} else {
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			val /= right
		}
	}
	return val, nil
}

// parseFactor handles unary minus, parentheses, numbers, and variable names.
func (p *exprParser) parseFactor() (int, error) {
	p.skipSpace()
	if p.pos >= len(p.input) {
		return 0, fmt.Errorf("unexpected end of expression")
	}

	ch := p.input[p.pos]

	// Unary minus
	if ch == '-' {
		p.depth++
		if p.depth > maxExprDepth {
			return 0, fmt.Errorf("expression too deeply nested (max depth %d)", maxExprDepth)
		}
		p.pos++
		val, err := p.parseFactor()
		p.depth--
		return -val, err
	}

	// Parenthesised sub-expression
	if ch == '(' {
		p.depth++
		if p.depth > maxExprDepth {
			return 0, fmt.Errorf("expression too deeply nested (max depth %d)", maxExprDepth)
		}
		p.pos++
		val, err := p.parseExpr()
		p.depth--
		if err != nil {
			return 0, err
		}
		p.skipSpace()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return 0, fmt.Errorf("expected ')'")
		}
		p.pos++
		return val, nil
	}

	// Integer literal
	if ch >= '0' && ch <= '9' {
		start := p.pos
		for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
			p.pos++
		}
		n, err := strconv.Atoi(p.input[start:p.pos])
		if err != nil {
			return 0, fmt.Errorf("invalid number: %w", err)
		}
		return n, nil
	}

	// Variable name: starts with letter or underscore; may contain letters, digits, underscore, dot
	if unicode.IsLetter(rune(ch)) || ch == '_' {
		start := p.pos
		for p.pos < len(p.input) {
			r := rune(p.input[p.pos])
			if unicode.IsLetter(r) || unicode.IsDigit(r) || p.input[p.pos] == '_' || p.input[p.pos] == '.' {
				p.pos++
			} else {
				break
			}
		}
		name := p.input[start:p.pos]
		if p.vars != nil {
			if v, ok := p.vars[name]; ok {
				return v, nil
			}
		}
		return 0, fmt.Errorf("undefined variable %q", name)
	}

	return 0, fmt.Errorf("unexpected character %q at position %d", ch, p.pos)
}
