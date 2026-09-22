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
	if delimiter == '{' && afterCommandIntroducer(line, index) {
		return true
	}
	// `function name {` -- the one place a `{` follows a bare word and still opens a
	// group. Everywhere else a brace after a word is data, which is what keeps
	// `echo {` printing a brace. Sixth layer to need telling about a construct, and
	// the reason the count is worth stating: see array.go.
	return delimiter == '{' && afterFunctionKeyword(line, index)
}

// afterCommandIntroducer reports whether everything before index is a reserved word a
// command may follow.
//
// **Every** word, not just the last: that is what lets `if ! { false; }` through -- two
// introducers in a row -- while keeping `echo if { a; }` out, where the `{` really is an
// argument. Both references refuse that second form too.
//
// Only back as far as the command position. The separator scan runs over a whole logical
// line before anything is cut, so `if true; then { echo a; }` reaches here with `if true;
// then ` in front of the brace -- and reading all of that would find `true;` and refuse.
// What decides is the words since the last separator, which is `then`.
//
// A separator inside quotes would be read as one, the way previousNonBlank beside this
// already does. `echo ";" if { a; }` is the shape that needs, and it is a syntax error in
// both references whichever way this answers.
func afterCommandIntroducer(line string, index int) bool {
	prefix := line[:index]
	for offset := len(prefix) - 1; offset >= 0; offset-- {
		if isCommandSeparator(prefix[offset]) {
			prefix = prefix[offset+1:]
			break
		}
	}
	fields := strings.Fields(prefix)
	if len(fields) == 0 {
		return false
	}
	for _, field := range fields {
		if !commandIntroducers[field] {
			return false
		}
	}
	return true
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
