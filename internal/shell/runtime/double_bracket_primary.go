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

// evaluateConditionTest is one test: its operands expanded, traced as bash traces a test, and
// answered. The unary tests and the numeric comparisons are `[`'s own code; -v and -o ask of
// the shell, and -a is -e, as it is inside `[[ ]]` in bash, where it cannot mean "and".
func (r Runtime) evaluateConditionTest(ctx context.Context, node *conditionNode, savedStatus int) (bool, error) {
	if node.kind == conditionUnary {
		operand, _ := r.expandConditionWord(ctx, node.operands[0], savedStatus)
		if r.shellErrorRaised() {
			return false, nil
		}
		r.traceConditionTest(ctx, node, []string{operand}, savedStatus)
		switch node.operator {
		case "-v":
			return r.variableIsSet(ctx, operand), nil
		case "-o":
			return r.ShellOptionIsOn(operand), nil
		case "-a":
			return applets.EvaluateConditionPrimary(r, "-e", operand, "")
		}
		return applets.EvaluateConditionPrimary(r, node.operator, operand, "")
	}
	left := conditionTerm{}
	left.text, left.quoted = r.expandConditionWord(ctx, node.operands[0], savedStatus)
	right := r.conditionOperandTerm(ctx, node.operator, node.operands[1], savedStatus)
	if r.shellErrorRaised() {
		return false, nil
	}
	r.traceConditionTest(ctx, node, []string{left.text, right.text}, savedStatus)
	return r.evaluateBinaryCondition(node.operator, left, right)
}

// conditionOperandTerm is a binary test's right side: a regular expression after `=~`, a
// pattern with literal parts after `==` or `!=` when some of it is quoted, and otherwise a
// word expanded whole.
func (r Runtime) conditionOperandTerm(ctx context.Context, operator string, item word, savedStatus int) conditionTerm {
	switch {
	case operator == "=~":
		return r.regexOperandTerm(ctx, item, savedStatus)
	case isPatternOperator(operator) && wordHasQuotedPart(item):
		return r.patternOperandTerm(ctx, item, savedStatus)
	}
	text, quoted := r.expandConditionWord(ctx, item, savedStatus)
	return conditionTerm{text: text, quoted: quoted}
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
