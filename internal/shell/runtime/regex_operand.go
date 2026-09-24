package runtime

import (
	"context"
	"regexp"
	"strings"
)

// regexOperandTerm expands the operand of `=~` with its quoted parts made literal, which is
// the other half of bash's rule: `[[ a.c =~ "a.c" ]]` is a match on a dot, `[[ abc =~
// "a.c" ]]` is not, and `[[ abc =~ "$re" ]]` matches $re's text rather than its pattern. A
// quoted part was a regular expression here, so each of those answered yes; with a working
// unquoted spelling for a group there is no longer a reason to keep that.
//
// Part by part, so an unquoted `$re` stays a pattern beside a quoted literal. Only for this
// operand: every other term is expanded once, as a whole, as it always was.
func (r Runtime) regexOperandTerm(ctx context.Context, item word, savedStatus int) conditionTerm {
	var text, source strings.Builder
	quoted := false
	for _, part := range item.parts {
		value := strings.Join(r.expandWord(ctx, word{parts: []wordPart{part}}, savedStatus), " ")
		text.WriteString(value)
		if part.quote != quoteUnquoted || part.kind == wordPartEscaped {
			quoted = true
			source.WriteString(regexp.QuoteMeta(value))
			continue
		}
		source.WriteString(value)
	}
	return conditionTerm{text: text.String(), quoted: quoted, regex: source.String(), hasRegex: true}
}

// The right side of `[[ x =~ regex ]]` is one word, parentheses and all.
//
// bash reads it to the first blank outside a bracket, so `[[ abc =~ ([a-z])(c) ]]` and
// `[[ $x =~ ^(on|yes)$ ]]` are regular expressions with groups in them. Here the group pass
// took the `(` for a subshell and reported `unexpected )`, and the only spelling that
// worked was a quoted one -- which bash 3.2 and later read as a *literal*, so the same line
// meant something else there (case_awareness.go recorded both halves of that).
//
// The lexer already reads the operand as one word inside a condition; it was only this pass,
// which runs before it, that had to be told. Only the operand of `=~`: `[[ ( a == a ) ]]`
// keeps its parentheses as grouping, which is where the earlier attempt went wrong.

// followsRegexOperator reports whether the text so far ends with `=~` and blanks, inside a
// `[[` -- so the next non-blank begins a regular expression.
func followsRegexOperator(before string) bool {
	trimmed := strings.TrimRight(before, " \t")
	if len(trimmed) == len(before) || !strings.HasSuffix(trimmed, "=~") {
		return false
	}
	if len(trimmed) > 2 && !isShellBlank(trimmed[len(trimmed)-3]) {
		return false
	}
	open := strings.LastIndex(trimmed, "[[")
	return open >= 0 && !strings.Contains(trimmed[open:], "]]")
}

// regexOperandEnd is where the operand starting at start ends: the first unquoted blank with
// every parenthesis and bracket closed, or the end of the line.
func regexOperandEnd(line string, start int) int {
	depth := 0
	quote := byte(0)
	for index := start; index < len(line); index++ {
		char := line[index]
		switch {
		case quote != 0:
			if char == quote {
				quote = 0
			} else if char == '\\' && quote == '"' {
				index++
			}
		case char == '\\':
			index++
		case char == '\'' || char == '"':
			quote = char
		case char == '(' || char == '[':
			depth++
		case char == ')' || char == ']':
			if depth > 0 {
				depth--
			}
		case isShellBlank(char) && depth == 0:
			return index
		}
	}
	return len(line)
}
