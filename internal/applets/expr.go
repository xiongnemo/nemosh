package applets

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// expr evaluates one expression and prints the result.
//
// POSIX's grammar, lowest precedence first, which is the whole substance of the
// utility -- an implementation that evaluates left to right gets `2 + 3 * 4`
// wrong and nothing else about it looks broken:
//
//	|            or
//	&            and
//	= > >= < <= !=   comparison
//	+ -
//	* / %
//	:            regular expression match
//
// Measured against GNU, which is where the exit status surprises people:
//
//	$ expr 2 + 3 \* 4     14
//	$ expr \( 2 + 3 \) \* 4   20
//	$ expr 0              0, and the status is 1
//
// The status is 0 when the result is neither empty nor zero, 1 when it is, and 2
// for a bad expression. So `expr` cannot be used with `set -e` to test a sum that
// might legitimately be zero, and that is not a bug here.
func newExprApplet() Applet {
	return simpleApplet{name: "expr", runContext: func(_ context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		if len(args) == 0 {
			return ExitStatusMessage(2, fmt.Errorf("missing operand"))
		}
		parser := &exprParser{tokens: args}
		value, err := parser.parseOr()
		if err != nil {
			return ExitStatusMessage(2, err)
		}
		if !parser.done() {
			return ExitStatusMessage(2, fmt.Errorf("syntax error: unexpected %s", parser.peek()))
		}
		fmt.Fprintln(stdout, value)
		if value == "" || value == "0" {
			return ExitStatus(1)
		}
		return nil
	}}
}

type exprParser struct {
	tokens []string
	at     int
}

func (p *exprParser) done() bool { return p.at >= len(p.tokens) }

func (p *exprParser) peek() string {
	if p.done() {
		return ""
	}
	return p.tokens[p.at]
}

func (p *exprParser) take() string {
	token := p.peek()
	p.at++
	return token
}

// Each level parses the one below it and then loops on its own operators, which
// is what makes precedence come out right without a table.
func (p *exprParser) parseOr() (string, error) {
	left, err := p.parseAnd()
	if err != nil {
		return "", err
	}
	for p.peek() == "|" {
		p.take()
		right, err := p.parseAnd()
		if err != nil {
			return "", err
		}
		// POSIX: the first operand if it is neither null nor zero, else the
		// second. Not a boolean -- `expr "" \| abc` is `abc`.
		if isTruthy(left) {
			continue
		}
		// The second operand, and `0` where that is null. Measured: GNU prints 0
		// for `expr "" \| ""` rather than an empty line, so a null result of a
		// logical operator is spelled as the number.
		left = right
		if left == "" {
			left = "0"
		}
	}
	return left, nil
}

func (p *exprParser) parseAnd() (string, error) {
	left, err := p.parseComparison()
	if err != nil {
		return "", err
	}
	for p.peek() == "&" {
		p.take()
		right, err := p.parseComparison()
		if err != nil {
			return "", err
		}
		if isTruthy(left) && isTruthy(right) {
			continue
		}
		left = "0"
	}
	return left, nil
}

var comparisons = map[string]bool{"=": true, "!=": true, "<": true, "<=": true, ">": true, ">=": true}

func (p *exprParser) parseComparison() (string, error) {
	left, err := p.parseAdditive()
	if err != nil {
		return "", err
	}
	for comparisons[p.peek()] {
		operator := p.take()
		right, err := p.parseAdditive()
		if err != nil {
			return "", err
		}
		left = compareExprValues(operator, left, right)
	}
	return left, nil
}

// compareExprValues compares numerically when both sides are numbers and as
// strings otherwise, which is POSIX's rule and the reason `expr 10 \> 9` is 1
// while `expr a10 \> a9` is 0.
func compareExprValues(operator, left, right string) string {
	leftNumber, leftIsNumber := exprNumber(left)
	rightNumber, rightIsNumber := exprNumber(right)
	var result bool
	if leftIsNumber && rightIsNumber {
		switch operator {
		case "=":
			result = leftNumber == rightNumber
		case "!=":
			result = leftNumber != rightNumber
		case "<":
			result = leftNumber < rightNumber
		case "<=":
			result = leftNumber <= rightNumber
		case ">":
			result = leftNumber > rightNumber
		case ">=":
			result = leftNumber >= rightNumber
		}
	} else {
		switch operator {
		case "=":
			result = left == right
		case "!=":
			result = left != right
		case "<":
			result = left < right
		case "<=":
			result = left <= right
		case ">":
			result = left > right
		case ">=":
			result = left >= right
		}
	}
	if result {
		return "1"
	}
	return "0"
}

func (p *exprParser) parseAdditive() (string, error) {
	left, err := p.parseMultiplicative()
	if err != nil {
		return "", err
	}
	for p.peek() == "+" || p.peek() == "-" {
		operator := p.take()
		right, err := p.parseMultiplicative()
		if err != nil {
			return "", err
		}
		if left, err = arithmetic(operator, left, right); err != nil {
			return "", err
		}
	}
	return left, nil
}

func (p *exprParser) parseMultiplicative() (string, error) {
	left, err := p.parseMatch()
	if err != nil {
		return "", err
	}
	for p.peek() == "*" || p.peek() == "/" || p.peek() == "%" {
		operator := p.take()
		right, err := p.parseMatch()
		if err != nil {
			return "", err
		}
		if left, err = arithmetic(operator, left, right); err != nil {
			return "", err
		}
	}
	return left, nil
}

// parseMatch is `:`, the anchored regular expression match, and the highest
// precedence binary operator.
func (p *exprParser) parseMatch() (string, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	for p.peek() == ":" {
		p.take()
		pattern, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		if left, err = matchExpr(left, pattern); err != nil {
			return "", err
		}
	}
	return left, nil
}

// matchExpr implements `STRING : BRE`.
//
// Anchored at the start, which is what makes `expr abc : 'a*'` answer 1 rather
// than 3: it counts the characters `a*` matched from the beginning, and that is
// one. With a capture group it yields the group's text instead of a count, which
// is the form scripts use to pull a version number out of a string.
func matchExpr(value, pattern string) (string, error) {
	compiled, err := regexp.Compile("^(?:" + basicToGoRegexp(pattern) + ")")
	if err != nil {
		return "", fmt.Errorf("invalid regular expression: %s", pattern)
	}
	found := compiled.FindStringSubmatch(value)
	if found == nil {
		if strings.Contains(pattern, `\(`) {
			return "", nil
		}
		return "0", nil
	}
	if len(found) > 1 {
		return found[1], nil
	}
	return strconv.Itoa(len(found[0])), nil
}

// basicToGoRegexp translates the BRE spellings expr takes into Go's syntax:
// `\(` and `\)` group, and a bare `(` is a literal.
func basicToGoRegexp(pattern string) string {
	var out strings.Builder
	for index := 0; index < len(pattern); index++ {
		if pattern[index] == '\\' && index+1 < len(pattern) {
			switch pattern[index+1] {
			case '(', ')', '{', '}':
				out.WriteByte(pattern[index+1])
				index++
				continue
			}
			out.WriteByte(pattern[index])
			out.WriteByte(pattern[index+1])
			index++
			continue
		}
		switch pattern[index] {
		case '(', ')', '{', '}':
			out.WriteString(regexp.QuoteMeta(string(pattern[index])))
		default:
			out.WriteByte(pattern[index])
		}
	}
	return out.String()
}

// parsePrimary is a parenthesised expression, one of the named functions, or a
// value.
func (p *exprParser) parsePrimary() (string, error) {
	// done() rather than an empty token, because an empty *argument* is a legal
	// operand: `expr "" \| abc` is `abc`, and conflating the two made that a
	// syntax error.
	if p.done() {
		return "", fmt.Errorf("syntax error: expression ended early")
	}
	switch token := p.peek(); token {
	case "(":
		p.take()
		inner, err := p.parseOr()
		if err != nil {
			return "", err
		}
		if p.take() != ")" {
			return "", fmt.Errorf("syntax error: missing )")
		}
		return inner, nil
	case "length", "substr", "index", "match":
		return p.parseFunction(token)
	}
	return p.take(), nil
}

// parseFunction covers GNU's named forms, which are not POSIX but are what people
// have in scripts: `length STRING`, `substr STRING POS LEN`, `index STRING CHARS`
// and `match STRING BRE`.
func (p *exprParser) parseFunction(name string) (string, error) {
	p.take()
	switch name {
	case "length":
		value, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		return strconv.Itoa(len([]rune(value))), nil
	case "match":
		value, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		pattern, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		return matchExpr(value, pattern)
	case "index":
		value, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		chars, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		return strconv.Itoa(strings.IndexAny(value, chars) + 1), nil
	}
	// substr: one-based, and out-of-range yields the empty string rather than an
	// error, which is what GNU does and what a script slicing a short value needs.
	value, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	position, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	length, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	start, startOk := exprNumber(position)
	count, countOk := exprNumber(length)
	runes := []rune(value)
	if !startOk || !countOk || start < 1 || count < 1 || int(start) > len(runes) {
		return "", nil
	}
	end := min(int(start-1+count), len(runes))
	return string(runes[start-1 : end]), nil
}

func arithmetic(operator, left, right string) (string, error) {
	leftNumber, leftOk := exprNumber(left)
	rightNumber, rightOk := exprNumber(right)
	if !leftOk || !rightOk {
		return "", fmt.Errorf("non-integer argument")
	}
	switch operator {
	case "+":
		return strconv.FormatInt(leftNumber+rightNumber, 10), nil
	case "-":
		return strconv.FormatInt(leftNumber-rightNumber, 10), nil
	case "*":
		return strconv.FormatInt(leftNumber*rightNumber, 10), nil
	}
	if rightNumber == 0 {
		return "", fmt.Errorf("division by zero")
	}
	if operator == "/" {
		return strconv.FormatInt(leftNumber/rightNumber, 10), nil
	}
	return strconv.FormatInt(leftNumber%rightNumber, 10), nil
}

func exprNumber(value string) (int64, bool) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed, err == nil
}

// isTruthy is expr's notion: neither empty nor the number zero.
func isTruthy(value string) bool {
	if value == "" {
		return false
	}
	if number, ok := exprNumber(value); ok {
		return number != 0
	}
	return true
}
