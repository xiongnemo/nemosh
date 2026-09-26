package runtime

import "strings"

// A conditional's parentheses group its expression: `[[ ! ( a == b ) ]]`, `[[ $x -gt 3 && (
// $x -lt 10 || $x -eq 99 ) ]]`. They are not commands, and nothing here knew it. The group
// pass took `( a == b )` for a subshell and left a placeholder word in its place, and a
// conditional's lone word is true when it is not empty -- so every parenthesised test was
// true, whatever it said, `[[ ( 1 -eq 2 ) ]]` included, and `!` in front of one made it
// false. The tests that pinned the grouping passed because the answer they wanted was true.
//
// So the passes that have an opinion about parentheses step over a whole conditional, and
// the lexer reads its parentheses as words of their own (lexedOperator), which is what the
// condition parser has always expected of them.

// conditionSpanEnd reports where the conditional that opens at index ends: the index just
// past its `]]`. There is none when `[[` is not the start of a command word there, or when
// nothing closes it, and then the text is left to the passes as it always was.
//
// The `]]` is the first one that is a word of its own, outside quotes and substitutions. It
// may follow a `)` directly, as in `[[ (a == a)]]`, since the parenthesis is a word too.
func conditionSpanEnd(line string, index int) (int, bool) {
	if !opensCondition(line, index) {
		return 0, false
	}
	quote := byte(0)
	for at := index + 2; at < len(line); at++ {
		char := line[at]
		if quote == '\'' {
			if char == '\'' {
				quote = 0
			}
			continue
		}
		switch {
		case char == '\\':
			at++
		case strings.HasPrefix(line[at:], "$(("):
			if end, ok := arithmeticExpansionEnd(line, at+3); ok {
				at = end
			}
		case strings.HasPrefix(line[at:], "$("):
			if end, ok := commandSubstitutionEnd(line, at+2); ok {
				at = end
			}
		case char == '"' && quote == '"':
			quote = 0
		case quote != 0:
		case char == '"' || char == '\'':
			quote = char
		case strings.HasPrefix(line[at:], "]]") && (isShellBlank(line[at-1]) || line[at-1] == ')') && endsConditionWord(line, at+2):
			return at + 2, true
		}
	}
	return 0, false
}

// opensCondition reports whether the `[[` at index begins a command word: at the start of the
// text or after a blank or a separator, and followed by a blank.
func opensCondition(line string, index int) bool {
	if !strings.HasPrefix(line[index:], "[[ ") && !strings.HasPrefix(line[index:], "[[\t") {
		return false
	}
	return index == 0 || isShellBlank(line[index-1]) || strings.IndexByte(";&|(){}!\n", line[index-1]) >= 0
}

// endsConditionWord reports whether a word can end at index: at the end of the text, or before
// a blank or a separator.
func endsConditionWord(line string, index int) bool {
	return index == len(line) || isShellBlank(line[index]) || strings.IndexByte(";&|)\n", line[index]) >= 0
}
