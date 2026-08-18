package runtime

import "strings"

// Where a brace is a reserved word rather than data. Split from parser_group.go to stay
// under the 250-line ceiling when the `function` keyword arrived.

// braceDelimiterAt reports whether the brace at index is the reserved word rather than
// an ordinary character, which is what keeps `echo {` printing a brace.

func braceDelimiterAt(line string, index int, delimiter byte) bool {
	if line[index] != delimiter {
		return false
	}
	if index+1 != len(line) && !isCommandBoundary(line[index+1]) {
		return false
	}
	previous, found := previousNonBlank(line, index)
	if !found || isCommandSeparator(previous) {
		return true
	}
	if delimiter == '{' && (previous == ')' || previous == '(' || previous == '{') {
		return true
	}
	// `function name {` -- the one place a `{` follows a bare word and still opens a
	// group. Everywhere else a brace after a word is data, which is what keeps
	// `echo {` printing a brace. Sixth layer to need telling about a construct, and
	// the reason the count is worth stating: see array.go.
	return delimiter == '{' && afterFunctionKeyword(line, index)
}

// afterFunctionKeyword reports whether the text before index is exactly
// `function name`.
func afterFunctionKeyword(line string, index int) bool {
	rest, ok := cutFunctionKeyword(strings.TrimSpace(line[:index]))
	if !ok {
		return false
	}
	_, valid := newFunctionName(strings.TrimSpace(rest))
	return valid
}
