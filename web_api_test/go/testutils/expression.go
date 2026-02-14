// Package testutils provides utilities for testing, including
// a simple expression evaluator for arithmetic expressions with variables.
package testutils

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ExprError represents an error during expression evaluation.
type ExprError struct {
	Message string
	Pos     int
}

func (e ExprError) Error() string {
	return fmt.Sprintf("expression error at position %d: %s", e.Pos, e.Message)
}

// Eval evaluates an arithmetic expression and returns the result.
// The expression may contain numbers, identifiers (variable names), operators
// +, -, *, /, parentheses, and whitespace. Variable values are provided in vars.
// Division by zero returns an error.
func Eval(expr string, vars map[string]float64) (float64, error) {
	p := &parser{
		src:    []rune(expr),
		pos:    0,
		vars:   vars,
	}
	result, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	if p.pos < len(p.src) {
		return 0, ExprError{Message: "unexpected trailing characters", Pos: p.pos}
	}
	return result, nil
}

type parser struct {
	src  []rune
	pos  int
	vars map[string]float64
}

func (p *parser) peek() rune {
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

func (p *parser) next() rune {
	if p.pos >= len(p.src) {
		return 0
	}
	r := p.src[p.pos]
	p.pos++
	return r
}

func (p *parser) skipWhitespace() {
	for p.pos < len(p.src) && unicode.IsSpace(p.src[p.pos]) {
		p.pos++
	}
}

func (p *parser) parseExpr() (float64, error) {
	return p.parseAddSub()
}

// parseAddSub handles + and -
func (p *parser) parseAddSub() (float64, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return 0, err
	}
	for {
		p.skipWhitespace()
		op := p.peek()
		if op != '+' && op != '-' {
			break
		}
		p.next() // consume op
		right, err := p.parseMulDiv()
		if err != nil {
			return 0, err
		}
		switch op {
		case '+':
			left += right
		case '-':
			left -= right
		}
	}
	return left, nil
}

// parseMulDiv handles * and /
func (p *parser) parseMulDiv() (float64, error) {
	left, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	for {
		p.skipWhitespace()
		op := p.peek()
		if op != '*' && op != '/' {
			break
		}
		p.next() // consume op
		right, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		switch op {
		case '*':
			left *= right
		case '/':
			if right == 0 {
				return 0, ExprError{Message: "division by zero", Pos: p.pos - 1}
			}
			left /= right
		}
	}
	return left, nil
}

// parseUnary handles unary + and -
func (p *parser) parseUnary() (float64, error) {
	p.skipWhitespace()
	op := p.peek()
	if op == '+' || op == '-' {
		p.next()
		val, err := p.parsePrimary()
		if err != nil {
			return 0, err
		}
		if op == '-' {
			val = -val
		}
		return val, nil
	}
	return p.parsePrimary()
}

// parsePrimary handles numbers, identifiers, and parentheses
func (p *parser) parsePrimary() (float64, error) {
	p.skipWhitespace()
	ch := p.peek()
	if ch == '(' {
		p.next() // consume '('
		val, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.skipWhitespace()
		if p.peek() != ')' {
			return 0, ExprError{Message: "expected ')'", Pos: p.pos}
		}
		p.next() // consume ')'
		return val, nil
	}
	if unicode.IsDigit(ch) || ch == '.' {
		return p.parseNumber()
	}
	if unicode.IsLetter(ch) {
		ident := p.parseIdent()
		val, ok := p.vars[ident]
		if !ok {
			return 0, ExprError{Message: fmt.Sprintf("undefined variable: %s", ident), Pos: p.pos - len([]rune(ident))}
		}
		return val, nil
	}
	return 0, ExprError{Message: "unexpected character", Pos: p.pos}
}

func (p *parser) parseNumber() (float64, error) {
	start := p.pos
	for p.pos < len(p.src) && (unicode.IsDigit(p.src[p.pos]) || p.src[p.pos] == '.') {
		p.pos++
	}
	numStr := string(p.src[start:p.pos])
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, ExprError{Message: "invalid number format", Pos: start}
	}
	return val, nil
}

func (p *parser) parseIdent() string {
	start := p.pos
	for p.pos < len(p.src) && (unicode.IsLetter(p.src[p.pos]) || unicode.IsDigit(p.src[p.pos]) || p.src[p.pos] == '_') {
		p.pos++
	}
	return string(p.src[start:p.pos])
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func TestEval(t *testing.T) {
//     vars := map[string]float64{"x": 10, "y": 5}
//     result, err := testutils.Eval("(x + y) * 2", vars)
//     if err != nil || result != 30 {
//         t.Errorf("got %v, want 30", result)
//     }
// }