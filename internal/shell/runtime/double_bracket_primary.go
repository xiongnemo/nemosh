package runtime

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// The primaries of `[[ ]]`: a parenthesised group, a unary test, a binary
// comparison, or a bare word.
//
// The unary tests and the numeric comparisons are the same set `[` implements, and
// they are evaluated through the same code -- two copies of `-f` would drift, and
// this project has fixed that class of bug twice already.

// doubleBracketBinaryOperators are the operators that take a left and a right.
var doubleBracketBinaryOperators = map[string]bool{
	"==": true, "=": true, "!=": true, "=~": true,
	"<": true, ">": true,
	"-eq": true, "-ne": true, "-lt": true, "-le": true, "-gt": true, "-ge": true,
	// File comparisons, which `[` has too.
	"-nt": true, "-ot": true, "-ef": true,
}

func (p *conditionParser) parsePrimary() (bool, error) {
	if p.done() {
		return false, fmt.Errorf("expression ended early")
	}
	if term := p.peek(); term.text == "(" && !term.quoted {
		p.take()
		value, err := p.parseOr()
		if err != nil {
			return false, err
		}
		if p.done() || p.peek().text != ")" {
			return false, fmt.Errorf("missing )")
		}
		p.take()
		return value, nil
	}
	// A unary operator is only a unary operator when something follows it, so
	// `[[ -n ]]` is a test of the string "-n" rather than a syntax error. bash
	// does the same, and it is why `[[ -n $x ]]` is safe when x is unset.
	if term := p.peek(); applets.IsUnaryConditionOperator(term.text) && !term.quoted && p.at+1 < len(p.terms) {
		operator := p.take().text
		operand := p.take()
		return applets.EvaluateConditionPrimary(p.runtime, operator, operand.text, "")
	}
	left := p.take()
	if p.done() {
		// A bare word is true when it is not empty, which is `[[ $x ]]`.
		return left.text != "", nil
	}
	operator := p.peek()
	if !doubleBracketBinaryOperators[operator.text] || operator.quoted {
		return left.text != "", nil
	}
	p.take()
	if p.done() {
		return false, fmt.Errorf("%s needs a right-hand side", operator.text)
	}
	right := p.take()
	return p.runtime.evaluateBinaryCondition(operator.text, left, right)
}

// evaluateBinaryCondition is where `[[ ]]` differs from `[` rather than merely
// looking different.
func (r Runtime) evaluateBinaryCondition(operator string, left, right conditionTerm) (bool, error) {
	switch operator {
	case "==", "=":
		// The right side is a *pattern* unless it was quoted. Measured:
		// `[[ abc == a* ]]` is true and `[[ abc == "a*" ]]` is false.
		if right.quoted {
			return left.text == right.text, nil
		}
		return matchShellPattern(right.text, left.text), nil
	case "!=":
		if right.quoted {
			return left.text != right.text, nil
		}
		return !matchShellPattern(right.text, left.text), nil
	case "=~":
		// An extended regular expression, anchored nowhere -- so `[[ abc =~ b ]]`
		// is true.
		//
		// A quoted right side is treated as a regular expression here, which bash 3.2
		// and later do not do: there, quoting makes it a literal string. The divergence
		// predates this comment and is recorded in case_awareness.go, where the reason it
		// has not been closed is set out -- an unquoted group does not reach the matcher
		// at all yet, so making quoting literal would leave no working spelling.
		expression, err := regexp.Compile(right.text)
		if err != nil {
			return false, fmt.Errorf("invalid regular expression: %s", right.text)
		}
		// FindStringSubmatch rather than MatchString, because the groups are the
		// point: `[[ $x =~ (a)(b) ]]` is how a script pulls fields out of a string
		// in bash, and the captures land in BASH_REMATCH. Without them the match
		// answered yes and then had nothing to show for it.
		return r.recordRegexMatch(expression.FindStringSubmatch(left.text)), nil
	case "<":
		return left.text < right.text, nil
	case ">":
		return left.text > right.text, nil
	}
	return r.evaluateConditionComparison(operator, left.text, right.text)
}

// evaluateConditionComparison covers the numeric and file operators, which are
// `[`'s and are evaluated by `[`'s own code.
func (r Runtime) evaluateConditionComparison(operator, left, right string) (bool, error) {
	switch operator {
	case "-eq", "-ne", "-lt", "-le", "-gt", "-ge":
		leftNumber, leftErr := strconv.ParseInt(left, 10, 64)
		rightNumber, rightErr := strconv.ParseInt(right, 10, 64)
		if leftErr != nil || rightErr != nil {
			return false, fmt.Errorf("%s: integer expression expected", operator)
		}
		switch operator {
		case "-eq":
			return leftNumber == rightNumber, nil
		case "-ne":
			return leftNumber != rightNumber, nil
		case "-lt":
			return leftNumber < rightNumber, nil
		case "-le":
			return leftNumber <= rightNumber, nil
		case "-gt":
			return leftNumber > rightNumber, nil
		}
		return leftNumber >= rightNumber, nil
	}
	// The file comparisons are `test`'s, evaluated by `test`'s own code.
	return applets.EvaluateConditionPrimary(r, operator, left, right)
}
