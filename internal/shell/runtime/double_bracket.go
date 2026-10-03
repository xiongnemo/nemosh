package runtime

import (
	"context"
	"fmt"
	"strings"
)

// `[[ ... ]]`, the conditional expression.
//
// Not POSIX -- dash has only `[` -- and this follows bash, measured:
//
//	x="a b"; [[ $x == "a b" ]]      true   -- no word splitting inside
//	[[ abc == a* ]]                 true   -- the right side is a pattern
//	[[ "abc" == "a*" ]]             false  -- quoted, so it is a literal
//	[[ abc =~ ^a.c$ ]]              true   -- a regular expression
//	[[ 3 -lt 5 ]]                   true
//	[[ 1 -eq 1 && 2 -eq 2 ]]        true
//	[[ -z $x || $(f) ]]             f runs only when x is not empty
//
// **The reason it exists is the first and the last of those.** Inside `[[ ]]` a
// word is not split and not globbed, so `[ $x = "a b" ]` -- which becomes
// `[ a b = a b ]` and is a syntax error -- works. That is the whole appeal, and
// it is also why this cannot be an applet: an applet receives words that have
// already been split, and by then the information is gone.
//
// So the expression is read with the script, the words still unexpanded (see
// double_bracket_parse.go), and each operand is expanded when the expression gets to
// it. That also supplies the other thing an applet could not know: whether the
// right-hand side of `==` was quoted, which decides pattern against literal.

// isDoubleBracket reports whether this command is a `[[ ]]` conditional: `[[` written
// plainly where the command begins, as the reserved word has to be.
func isDoubleBracket(command []word) bool {
	return len(command) > 0 && isUnquotedLiteralWord(command[0]) && soleLiteralText(command[0]) == "[["
}

// runDoubleBracket runs a conditional whose expression was not read with the script -- one a
// command built from words -- by reading it now. A syntax error is 2, as the parse's is.
func (r Runtime) runDoubleBracket(ctx context.Context, command []word, operations []redirectOperation, savedStatus int) lineResult {
	condition, err := parseDoubleBracket(command)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
		return lineResult{status: 2}
	}
	return r.runConditionCommand(ctx, condition, operations, savedStatus)
}

// runConditionCommand runs a `[[ ]]` command: its redirections, made as any command's are --
// `[[ -r $f ]] 2>/dev/null` and `[[ $x ]] > log` create their files in both references --
// and then its expression.
func (r Runtime) runConditionCommand(ctx context.Context, condition *conditionNode, operations []redirectOperation, savedStatus int) lineResult {
	// A `<(command)` in an operand keeps its file until the test is done; see runParsedWords.
	defer r.cleanUpProcessSubstitutions()
	expanded, ok := r.expandRedirectOperations(ctx, operations, savedStatus)
	if r.shellErrorRaised() {
		return r.shellErrorResult()
	}
	if !ok {
		return lineResult{status: 1}
	}
	return r.withAppliedRedirectsFor(false, expanded, func(redirected Runtime) lineResult {
		return redirected.runCondition(ctx, condition, savedStatus)
	})
}

// runCondition answers the expression: 0 for true, 1 for false, and 2 when a test cannot be
// made -- which is bash's, and keeps "the answer is no" apart from "that was no answer".
func (r Runtime) runCondition(ctx context.Context, condition *conditionNode, savedStatus int) lineResult {
	value, err := r.evaluateCondition(ctx, condition, savedStatus)
	if r.shellErrorRaised() {
		return r.shellErrorResult()
	}
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s[[: %v\n", r.diagnosticPrefix(), err)
		return lineResult{status: 2}
	}
	if value {
		return lineResult{}
	}
	return lineResult{status: 1}
}

// evaluateCondition is execute_cond_node: `&&` and `||` take their right side only when the
// left has not decided the answer, so an operand there is expanded only when it is wanted.
func (r Runtime) evaluateCondition(ctx context.Context, node *conditionNode, savedStatus int) (bool, error) {
	var value bool
	var err error
	switch node.kind {
	case conditionAnd, conditionOr:
		value, err = r.evaluateCondition(ctx, node.left, savedStatus)
		if err == nil && !r.shellErrorRaised() && value == (node.kind == conditionAnd) {
			value, err = r.evaluateCondition(ctx, node.right, savedStatus)
		}
	case conditionGroup:
		value, err = r.evaluateCondition(ctx, node.left, savedStatus)
	default:
		value, err = r.evaluateConditionTest(ctx, node, savedStatus)
	}
	return value != node.negated, err
}

// conditionTerm is one word of the expression, and whether any of it was quoted.
type conditionTerm struct {
	text   string
	quoted bool
	// regex is the term as a regular expression, its quoted parts made literal; set only
	// for the operand of `=~`. See regexOperandTerm.
	regex    string
	hasRegex bool
	// pattern is the term as a pattern, its quoted parts made literal; set only for a
	// partly quoted operand of `==` or `!=`. See patternOperandTerm.
	pattern    string
	hasPattern bool
}

// isPatternOperator reports `==`, `=` or `!=`, whose right side is a pattern.
func isPatternOperator(operator string) bool {
	return operator == "==" || operator == "=" || operator == "!="
}

// patternOperandTerm is the right side of `==` or `!=` when some of it is quoted: a
// pattern where it is unquoted and a literal where it is quoted, part by part, as a case
// arm's is -- so `[[ abc == "$p"* ]]` asks whether abc begins with what p holds. The word
// was compared literally as a whole, and that was false. Each part is expanded once.
func (r Runtime) patternOperandTerm(ctx context.Context, item word, savedStatus int) conditionTerm {
	expander := r.expandingAssignment()
	var text, pattern strings.Builder
	for _, part := range item.parts {
		value := strings.Join(expander.expandWord(ctx, word{parts: []wordPart{part}}, savedStatus), "")
		text.WriteString(value)
		if part.quote != quoteUnquoted || part.kind == wordPartEscaped {
			value = literalIn(operandPattern, value)
		}
		pattern.WriteString(value)
	}
	return conditionTerm{text: text.String(), quoted: true, pattern: pattern.String(), hasPattern: true}
}

// expandConditionWord expands one word with neither field splitting nor pathname
// expansion, and reports whether any part of it was quoted.
//
// The quoting matters for exactly one thing and it is not cosmetic: `[[ abc ==
// a* ]]` is a pattern match and `[[ abc == "a*" ]]` is a string comparison.
// Measured -- the second is false in bash.
func (r Runtime) expandConditionWord(ctx context.Context, item word, savedStatus int) (string, bool) {
	// Not split, so a value keeps its blanks: with x="a  b", `[[ $x == "a  b" ]]` is true. It
	// was split and joined again with one blank, and false. "$@" is its fields, joined.
	fields := r.expandingAssignment().expandWord(ctx, item, savedStatus)
	return strings.Join(fields, " "), wordHasQuotedPart(item)
}

// soleLiteralText is a word's text when it is one plain literal part, and "" for
// anything else. `[[` has to be recognised as written rather than as expanded: a
// variable holding the string `[[` is an ordinary command word.
func soleLiteralText(item word) string {
	if len(item.parts) != 1 || item.parts[0].kind != wordPartLiteral {
		return ""
	}
	return item.parts[0].text
}
