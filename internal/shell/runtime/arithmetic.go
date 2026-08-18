package runtime

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// evaluateArithmetic is the `$(( ))` expansion of POSIX 2.6.4: signed integer
// arithmetic over the C operator set, with a bare name standing for the value
// of that shell variable and an unset or non-numeric one standing for zero.
// Assignment writes back to the shell variable, so `$((i += 1))` both yields
// the new value and stores it.
func (r Runtime) evaluateArithmetic(expression string) (int64, error) {
	parser := &arithmeticParser{tokens: tokenizeArithmetic(expression), runtime: r}
	value, err := parser.assignment()
	if err != nil {
		return 0, err
	}
	if parser.index < len(parser.tokens) {
		return 0, fmt.Errorf("arithmetic syntax error: unexpected %q", parser.tokens[parser.index])
	}
	return value, nil
}

// assignment is the lowest-precedence level and the only one that writes:
// `x = expr` and the compound forms POSIX 2.6.4 lists. Right-associative, so
// `a = b = 1` sets both.
func (p *arithmeticParser) assignment() (int64, error) {
	name, operator, ok := p.assignmentTarget()
	if !ok {
		return p.ternary()
	}
	value, err := p.assignment()
	if err != nil {
		return 0, err
	}
	if operator != "=" {
		current, _ := p.lookup(name)
		if value, err = applyArithmetic(current, strings.TrimSuffix(operator, "="), value); err != nil {
			return 0, err
		}
	}
	p.runtime.vars[name] = strconv.FormatInt(value, 10)
	p.runtime.markVarMutation(name)
	return value, nil
}

// assignmentTarget consumes `name OP` when that is what comes next, and leaves
// the parser untouched when it is not -- `x == 1` is a comparison, and only the
// two-token lookahead can tell it from `x = 1`.
func (p *arithmeticParser) assignmentTarget() (string, string, bool) {
	if p.index+1 >= len(p.tokens) || !isVariableName(p.tokens[p.index]) {
		return "", "", false
	}
	operator := p.tokens[p.index+1]
	if !slices.Contains(arithmeticAssignmentOperators, operator) {
		return "", "", false
	}
	name := p.tokens[p.index]
	p.index += 2
	return name, operator, true
}

var arithmeticAssignmentOperators = []string{
	"=", "+=", "-=", "*=", "/=", "%=", "<<=", ">>=", "&=", "^=", "|=",
}

type arithmeticParser struct {
	tokens  []string
	index   int
	runtime Runtime
}

// An unset or non-numeric name reads as zero, which is what POSIX says of a
// variable with an unset or null value.
func (p *arithmeticParser) lookup(name string) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(p.runtime.vars[name]), 0, 64)
	return value, err == nil
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
		if !slices.Contains(arithmeticPrecedence[level], operator) {
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
	// `++i` and `--i`: the value after the change. Before the sign operators,
	// because `++` used to lex as two `+` and be read as unary plus applied twice
	// -- which returned the variable unchanged and reported nothing. `$((++i))`
	// gave 0 for an i of 0, silently.
	if operator := p.peek(); operator == "++" || operator == "--" {
		p.index++
		return p.step(operator, false)
	}
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
	if value, err := strconv.ParseInt(token, 0, 64); err == nil {
		return value, nil
	}
	if !isVariableName(token) {
		return 0, fmt.Errorf("arithmetic syntax error: unexpected %q", token)
	}
	value, _ := p.lookup(token)
	// `i++` and `i--`: the value before the change. This is what `for ((i=0;
	// i<n; i++))` is written with, and the difference from the prefix form is the
	// value the expression has, not what the variable ends up holding.
	if operator := p.peek(); operator == "++" || operator == "--" {
		p.index++
		if err := p.store(token, stepped(value, operator)); err != nil {
			return 0, err
		}
		return value, nil
	}
	return value, nil
}
