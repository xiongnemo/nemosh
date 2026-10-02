package runtime

import "strings"

// The extended pattern operators: `?(list)`, `*(list)`, `+(list)`, `@(list)`, `!(list)`.
//
//	rm !(keep.txt)                         everything but that file
//	case $f in @(*.jpg|*.png)) ... ;;      one of these
//	${x%%+([0-9])}                         strip a run of digits
//
// They were a syntax error, and `shopt -s extglob` -- which sits near the top of a great
// many scripts -- was refused by name.
//
// Recognised unconditionally rather than behind the option, which is a decision worth
// stating. bash gates them because it has decades of scripts where `@(a)` was a literal;
// this shell has no such legacy, and a pattern that means those five characters literally
// can still say so by quoting them. The alternative was threading the option through
// eleven call sites that all reach the matcher from a Runtime, to gate something nobody
// wants off. `shopt -s extglob` therefore succeeds and `shopt` reports it on; see
// builtin_shopt.go for what `-u extglob` does.
//
// A separate matcher from matchRunePattern, and recursive where that one is iterative.
// The plain operators need one backtrack point -- the last `*` -- and these need a real
// search: `+(a|ab)` against `aab` has to try both alternatives at both positions. Keeping
// them apart means an ordinary pattern, which is nearly all of them, still takes the
// cheap path.

// extendedOpeners are the characters that make a `(` an extended group.
const extendedOpeners = "?*+@!"

// hasExtendedPattern reports whether a pattern uses any of the operators. Cheap, because
// it runs on every pattern match to decide which matcher to use.
func hasExtendedPattern(pattern string) bool {
	for index := 0; index+1 < len(pattern); index++ {
		if pattern[index] == '\\' {
			index++
			continue
		}
		if pattern[index+1] == '(' && strings.IndexByte(extendedOpeners, pattern[index]) >= 0 {
			return true
		}
	}
	return false
}

// extendedGroupOpensAt reports whether the `(` at index belongs to an extended pattern
// operator rather than opening a subshell or a group.
//
// The test is the character before it, which is the whole of what distinguishes them:
// `@(a|b)` is a pattern and `(a)` is a subshell. Local and precise, which matters --
// trying to decide it from "are we inside `[[ ]]`" instead broke the nested condition
// form, and the reason is in case_awareness.go.
func extendedGroupOpensAt(line string, index int) bool {
	if index == 0 || index >= len(line) || line[index] != '(' {
		return false
	}
	if index >= 2 && line[index-2] == '\\' {
		// The operator itself was escaped, so it is data.
		return false
	}
	// A `!(` where a command begins is a negated subshell; see bangSubshellAt.
	if line[index-1] == '!' && bangSubshellAt(line, index-1) {
		return false
	}
	return strings.IndexByte(extendedOpeners, line[index-1]) >= 0
}

// skipBalancedParens returns the index just past the `)` that closes the `(` at index, and the
// line's end when none does. A parenthesis quoted inside is the group's text, as in `<(echo "a
// )")` and `@("a)"|b)`: every scan that stepped over a group with this took it for the close.
func skipBalancedParens(line string, index int) int {
	if end, closed := matchingParenthesis(line, index); closed {
		return end + 1
	}
	return len(line)
}

// extendedGroupLiterals are the characters that inside a group are the pattern's own, where
// outside one they would end the word: the pipe, the blanks, the other operators, and the
// parentheses, which nest.
const extendedGroupLiterals = "()| \t\n&;<>"

// extendedGroupDepth is how many of a group's parentheses are open once char is read.
func extendedGroupDepth(depth int, char byte) int {
	switch char {
	case '(':
		return depth + 1
	case ')':
		return depth - 1
	}
	return depth
}
