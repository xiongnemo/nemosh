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
	// A `}` may close its group right before a subshell's `)`: `({ :; })`.
	if index+1 != len(line) && !isCommandBoundary(line[index+1]) && (delimiter != '}' || line[index+1] != ')') {
		return false
	}
	previous, found := previousNonBlank(line, index)
	if !found || isCommandSeparator(previous) {
		return true
	}
	// A `}` right after another group's `}` closes its own group too, as POSIX has it and both
	// references read it: `f() { { echo in; } }` was "missing }". Only after a `}` that is
	// itself a delimiter, so `echo } }` still prints two braces.
	if delimiter == '}' && previous == '}' && braceDelimiterAt(line, previousNonBlankIndex(line, index), '}') {
		return true
	}
	if delimiter == '}' && afterCompoundCloser(line, index) {
		return true
	}
	// And right after one, which ends a command as a separator does: `{(true)}`, `{ f()(echo x)}`.
	if delimiter == '}' && previous == ')' && closesSubshellAt(line, previousNonBlankIndex(line, index)) {
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
	return delimiter == '{' && (afterFunctionKeyword(line, index) || afterCoprocKeyword(line, index))
}

// isCommandSeparator reports whether a character ends one command and so leaves
// the next byte in command position.
func isCommandSeparator(char byte) bool {
	return char == ';' || char == '&' || char == '|' || char == '\n'
}

// previousNonBlank reports the last character before index that is not a blank,
// and whether the scan found one before running off the front of the line.
func previousNonBlank(line string, index int) (byte, bool) {
	if back := previousNonBlankIndex(line, index); back >= 0 {
		return line[back], true
	}
	return 0, false
}

func isCommandBoundary(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '|' || char == '&' || char == '(' || char == '{' || char == ';'
}

// previousNonBlankIndex is where previousNonBlank's byte is, and -1 when there is none.
func previousNonBlankIndex(line string, index int) int {
	for back := index - 1; back >= 0; back-- {
		if line[back] != ' ' && line[back] != '\t' {
			return back
		}
	}
	return -1
}

// afterCompoundCloser reports a `}` right after the `fi`, `done` or `esac` that ends an if, a
// loop or a case -- one with a separator before it, so not `echo fi }` -- which is where a
// reserved word may stand, as both references read it: `{ if x; then y; fi }`.
func afterCompoundCloser(line string, index int) bool {
	end := previousNonBlankIndex(line, index)
	start := end
	for start >= 0 && !isCommandBoundary(line[start]) {
		start--
	}
	switch line[start+1 : end+1] {
	case "fi", "done", "esac":
		previous, found := previousNonBlank(line, start+1)
		return !found || isCommandSeparator(previous)
	}
	return false
}

// afterCoprocKeyword reports whether the words since the last separator are `coproc` or
// `coproc NAME`, after which bash's brace opens the coprocess's group. The form is refused
// when it runs (coproc.go), and read as a group here so the refusal is what arrives rather
// than `unexpected }`.
func afterCoprocKeyword(line string, index int) bool {
	prefix := line[:index]
	for offset := len(prefix) - 1; offset >= 0; offset-- {
		if isCommandSeparator(prefix[offset]) {
			prefix = prefix[offset+1:]
			break
		}
	}
	fields := strings.Fields(prefix)
	keyword := len(fields) - 1
	if keyword > 0 && fields[keyword] != "coproc" && isVariableName(fields[keyword]) {
		keyword--
	}
	if keyword < 0 || fields[keyword] != "coproc" {
		return false
	}
	// `then coproc {`, `do coproc {`, and `f() { coproc NAME {`: what stands before the
	// keyword is a reserved word a command may follow, or nothing. Only the word just
	// before, since a function's header is not one: the whole of `f() {` was asked, and
	// the brace of a coprocess in a one-line function body read as data.
	return keyword == 0 || commandIntroducers[fields[keyword-1]]
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
	// A group's close ends a command too, so a reserved word after one begins the next: the
	// `then` in `if { a; } then { b; }` introduced nothing, and its brace was data.
	for len(fields) > 0 && (fields[0] == "}" || fields[0] == ")") {
		fields = fields[1:]
	}
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

// afterFunctionKeyword reports whether the command before index is exactly `function name`:
// the text since the last separator, past a `then`, `do` or `else` that begins it. The whole
// line was asked, so `: prefix; function f {` and `then function g {` were "missing }",
// where busybox-w32 and bash both define the function.
func afterFunctionKeyword(line string, index int) bool {
	command := strings.TrimSpace(line[strings.LastIndexAny(line[:index], ";&|\n(){")+1 : index])
	for _, keyword := range [...]string{"then", "do", "else"} {
		if rest, ok := compoundHeader(command, keyword); ok {
			command = strings.TrimLeft(rest, " \t")
			break
		}
	}
	rest, ok := cutFunctionKeyword(command)
	if !ok {
		return false
	}
	_, valid := newFunctionName(strings.TrimSpace(rest))
	return valid
}

// closesSubshellAt reports whether the `)` at index closes a subshell -- a `(` where a command
// begins, a function's body among them -- rather than a substitution's, an array's or a
// pattern's, whose `)` a word goes on past: `{ echo $(date)}` is still an open group.
func closesSubshellAt(line string, index int) bool {
	for open := strings.LastIndexByte(line[:index], '('); open >= 0; open = strings.LastIndexByte(line[:open], '(') {
		if end, closed := matchingParenthesis(line, open); closed && end == index {
			previous, found := previousNonBlank(line, open)
			return !found || isCommandSeparator(previous) || strings.IndexByte("({)", previous) >= 0 || afterCommandIntroducer(line, open)
		}
	}
	return false
}
