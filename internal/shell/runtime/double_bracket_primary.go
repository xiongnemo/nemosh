package runtime

import (
	"context"
	"fmt"
	"regexp"

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
	// A binary operator after this term makes it a comparison, whatever the term looks like:
	// with x=-f, `[[ $x == $x ]]` compares two strings rather than asking whether a file named
	// `==` exists, and `[[ $p == "(" ]]` with p='(' compares two parentheses rather than
	// opening a group. It did both, and the leftover term was a syntax error. busybox and bash
	// agree; POSIX settles test's three-argument form the same way, on its middle word first.
	if p.binaryFollows() {
		left, operator, right := p.take(), p.take(), p.take()
		return p.runtime.evaluateBinaryCondition(operator.text, left, right)
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
	if term := p.peek(); term.text == "-v" && !term.quoted && p.at+1 < len(p.terms) {
		p.take()
		return p.runtime.variableIsSet(context.Background(), p.take().text), nil
	}
	if term := p.peek(); term.text == "-o" && !term.quoted && p.at+1 < len(p.terms) {
		p.take()
		return p.runtime.ShellOptionIsOn(p.take().text), nil
	}
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

// matchesOperand is `==`'s answer: a pattern match against a right side that is unquoted,
// or partly quoted, and a comparison against one that is quoted whole.
func (r Runtime) matchesOperand(left, right conditionTerm) bool {
	switch {
	case right.hasPattern:
		return r.matchWordPattern(right.pattern, left.text)
	case right.quoted:
		return r.equalWords(left.text, right.text)
	}
	return r.matchWordPattern(right.text, left.text)
}

// evaluateBinaryCondition is where `[[ ]]` differs from `[` rather than merely
// looking different.
func (r Runtime) evaluateBinaryCondition(operator string, left, right conditionTerm) (bool, error) {
	switch operator {
	case "==", "=":
		// The right side is a *pattern* unless it was quoted. Measured:
		// `[[ abc == a* ]]` is true and `[[ abc == "a*" ]]` is false.
		return r.matchesOperand(left, right), nil
	case "!=":
		return !r.matchesOperand(left, right), nil
	case "=~":
		// An extended regular expression, anchored nowhere -- so `[[ abc =~ b ]]`
		// is true. A quoted part of it is literal, as bash 3.2 and later have it; see
		// regexOperandTerm, and regex_operand.go for the unquoted group that made that
		// possible to fix.
		source := right.text
		if right.hasRegex {
			source = right.regex
		}
		source, err := posixIntervals(source)
		if err != nil {
			return false, fmt.Errorf("invalid regular expression: %s", right.text)
		}
		if r.noCaseMatch() {
			source = "(?i)" + source
		}
		expression, err := regexp.Compile(source)
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

// evaluateConditionComparison covers the numeric and file operators, which are `[`'s and are
// evaluated by `[`'s own code. The numeric ones were read here, and an operand that is no
// number was said of the operator, `[[: -lt: integer expression expected`, where `[` names
// the operand, `x: bad number`, as busybox's [[ does; and `[[ " 3" -eq 3 ]]`, which `[` and
// both references take, was refused.
func (r Runtime) evaluateConditionComparison(operator, left, right string) (bool, error) {
	return applets.EvaluateConditionPrimary(r, operator, left, right)
}
