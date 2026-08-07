package runtime

import (
	"fmt"
	"strconv"
	"strings"
)

// evaluateArithmetic is the `$(( ))` expansion of POSIX 2.6.4: signed integer
// arithmetic over the C operator set, with a bare name standing for the value
// of that shell variable and an unset or non-numeric one standing for zero.
//
// Assignment inside the expansion is deliberately absent and refused by name
// rather than mis-parsed, because `i=$((i+1))` -- the form that does not need
// it -- is the one Nemosh promises.
func (r Runtime) evaluateArithmetic(expression string) (int64, error) {
	parser := &arithmeticParser{tokens: tokenizeArithmetic(expression), vars: r.vars}
	value, err := parser.ternary()
	if err != nil {
		return 0, err
	}
	if parser.index < len(parser.tokens) {
		// The leftover is where an assignment shows up: the name parses as a
		// value and the `=` after it has nowhere to go. Naming it beats
		// reporting an unexpected token and leaving the reader to work out why.
		if parser.tokens[parser.index] == "=" {
			return 0, fmt.Errorf("arithmetic assignment is not supported")
		}
		return 0, fmt.Errorf("arithmetic syntax error: unexpected %q", parser.tokens[parser.index])
	}
	return value, nil
}

type arithmeticParser struct {
	tokens []string
	index  int
	vars   map[string]string
}

// The binary operators in order of increasing precedence, which is the order C
// gives them and the order POSIX adopts.
var arithmeticPrecedence = [][]string{
	{"||"},
	{"&&"},
	{"|"},
	{"^"},
	{"&"},
	{"==", "!="},
	{"<=", ">=", "<", ">"},
	{"<<", ">>"},
	{"+", "-"},
	{"*", "/", "%"},
}

func (p *arithmeticParser) ternary() (int64, error) {
	condition, err := p.binary(0)
	if err != nil || p.peek() != "?" {
		return condition, err
	}
	p.index++
	whenTrue, err := p.ternary()
	if err != nil {
		return 0, err
	}
	if p.peek() != ":" {
		return 0, fmt.Errorf("arithmetic syntax error: expected :")
	}
	p.index++
	whenFalse, err := p.ternary()
	if err != nil {
		return 0, err
	}
	if condition != 0 {
		return whenTrue, nil
	}
	return whenFalse, nil
}

func (p *arithmeticParser) binary(level int) (int64, error) {
	if level >= len(arithmeticPrecedence) {
		return p.unary()
	}
	left, err := p.binary(level + 1)
	if err != nil {
		return 0, err
	}
	for {
		operator := p.peek()
		if !containsString(arithmeticPrecedence[level], operator) {
			return left, nil
		}
		p.index++
		// && and || stop early, which matters because the right side may
		// divide by zero.
		if operator == "&&" && left == 0 || operator == "||" && left != 0 {
			if _, err := p.binary(level + 1); err != nil {
				return 0, err
			}
			continue
		}
		right, err := p.binary(level + 1)
		if err != nil {
			return 0, err
		}
		if left, err = applyArithmetic(left, operator, right); err != nil {
			return 0, err
		}
	}
}

func (p *arithmeticParser) unary() (int64, error) {
	switch operator := p.peek(); operator {
	case "-", "+", "!", "~":
		p.index++
		value, err := p.unary()
		if err != nil {
			return 0, err
		}
		switch operator {
		case "-":
			return -value, nil
		case "!":
			return boolValue(value == 0), nil
		case "~":
			return ^value, nil
		}
		return value, nil
	}
	return p.primary()
}

func (p *arithmeticParser) primary() (int64, error) {
	token := p.peek()
	if token == "" {
		return 0, fmt.Errorf("arithmetic syntax error: expression ended early")
	}
	p.index++
	if token == "(" {
		value, err := p.ternary()
		if err != nil {
			return 0, err
		}
		if p.peek() != ")" {
			return 0, fmt.Errorf("arithmetic syntax error: expected )")
		}
		p.index++
		return value, nil
	}
	if token == "=" {
		return 0, fmt.Errorf("arithmetic assignment is not supported")
	}
	if value, err := strconv.ParseInt(token, 0, 64); err == nil {
		return value, nil
	}
	if !isVariableName(token) {
		return 0, fmt.Errorf("arithmetic syntax error: unexpected %q", token)
	}
	// An unset or non-numeric name is zero, which is what POSIX says of a
	// variable with an unset or null value.
	value, _ := strconv.ParseInt(strings.TrimSpace(p.vars[token]), 0, 64)
	return value, nil
}

func (p *arithmeticParser) peek() string {
	if p.index >= len(p.tokens) {
		return ""
	}
	return p.tokens[p.index]
}

func applyArithmetic(left int64, operator string, right int64) (int64, error) {
	switch operator {
	case "+":
		return left + right, nil
	case "-":
		return left - right, nil
	case "*":
		return left * right, nil
	case "/", "%":
		if right == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		if operator == "/" {
			return left / right, nil
		}
		return left % right, nil
	case "<<":
		return left << uint64(right), nil
	case ">>":
		return left >> uint64(right), nil
	case "<":
		return boolValue(left < right), nil
	case "<=":
		return boolValue(left <= right), nil
	case ">":
		return boolValue(left > right), nil
	case ">=":
		return boolValue(left >= right), nil
	case "==":
		return boolValue(left == right), nil
	case "!=":
		return boolValue(left != right), nil
	case "&":
		return left & right, nil
	case "^":
		return left ^ right, nil
	case "|":
		return left | right, nil
	case "&&":
		return boolValue(left != 0 && right != 0), nil
	default:
		return boolValue(left != 0 || right != 0), nil
	}
}

func boolValue(condition bool) int64 {
	if condition {
		return 1
	}
	return 0
}

func containsString(set []string, value string) bool {
	for _, item := range set {
		if item == value {
			return true
		}
	}
	return false
}
