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

// matchExtendedPattern is matchShellPattern for a pattern that uses the operators.
func matchExtendedPattern(pattern, value []rune) bool {
	if len(pattern) == 0 {
		return len(value) == 0
	}
	if group, ok := extendedGroupAt(pattern, 0); ok {
		return matchExtendedGroup(group, pattern[group.end:], value)
	}
	if pattern[0] == '*' {
		// Every split, shortest first, so the search terminates on the empty one.
		for taken := 0; taken <= len(value); taken++ {
			if matchExtendedPattern(pattern[1:], value[taken:]) {
				return true
			}
		}
		return false
	}
	if len(value) == 0 {
		return false
	}
	next, ok := matchOneRune(pattern, 0, value[0])
	if !ok {
		return false
	}
	return matchExtendedPattern(pattern[next:], value[1:])
}

// extendedGroup is a parsed `X(a|b|c)`.
type extendedGroup struct {
	operator     rune
	alternatives [][]rune
	// end is where the pattern continues, just past the closing parenthesis.
	end int
}

// extendedGroupAt reads a group at index, reporting whether one is there.
func extendedGroupAt(pattern []rune, index int) (extendedGroup, bool) {
	if index+1 >= len(pattern) || pattern[index+1] != '(' {
		return extendedGroup{}, false
	}
	if !strings.ContainsRune(extendedOpeners, pattern[index]) {
		return extendedGroup{}, false
	}
	depth := 0
	for scan := index + 1; scan < len(pattern); scan++ {
		switch pattern[scan] {
		case '\\':
			scan++
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return extendedGroup{
					operator:     pattern[index],
					alternatives: splitAlternatives(pattern[index+2 : scan]),
					end:          scan + 1,
				}, true
			}
		}
	}
	// No closing parenthesis: not a group, and the characters are ordinary. POSIX says
	// the same about an unmatched bracket.
	return extendedGroup{}, false
}

// splitAlternatives cuts a list on its top-level `|`, so `@(a|b(c|d))` has two.
func splitAlternatives(list []rune) [][]rune {
	var alternatives [][]rune
	depth, start := 0, 0
	for index := 0; index < len(list); index++ {
		switch list[index] {
		case '\\':
			index++
		case '(':
			depth++
		case ')':
			depth--
		case '|':
			if depth == 0 {
				alternatives = append(alternatives, list[start:index])
				start = index + 1
			}
		}
	}
	return append(alternatives, list[start:])
}

// matchExtendedGroup matches one group against the front of value and the rest of the
// pattern against what is left.
func matchExtendedGroup(group extendedGroup, rest, value []rune) bool {
	switch group.operator {
	case '@':
		return matchGroupRepeated(group, rest, value, 1, 1)
	case '?':
		return matchGroupRepeated(group, rest, value, 0, 1)
	case '*':
		return matchGroupRepeated(group, rest, value, 0, -1)
	case '+':
		return matchGroupRepeated(group, rest, value, 1, -1)
	case '!':
		return matchGroupNegated(group, rest, value)
	}
	return false
}

// matchGroupRepeated matches the alternatives between least and most times, where -1 is
// no limit, and then the rest of the pattern.
//
// Written as a search over prefixes rather than as a loop that consumes greedily, because
// greedy consumption gets `+(a|ab)` against `aab` wrong: it takes `a`, then `a`, then has
// `b` left over and no way back.
func matchGroupRepeated(group extendedGroup, rest, value []rune, least, most int) bool {
	if least <= 0 && matchExtendedPattern(rest, value) {
		return true
	}
	if most == 0 {
		return false
	}
	for _, alternative := range group.alternatives {
		// Every prefix the alternative could account for. A prefix of length zero is
		// skipped, or a `*(a)` would recurse for ever on the same position.
		for taken := 1; taken <= len(value); taken++ {
			if !matchExtendedPattern(alternative, value[:taken]) {
				continue
			}
			remaining := most
			if remaining > 0 {
				remaining--
			}
			if matchGroupRepeated(group, rest, value[taken:], least-1, remaining) {
				return true
			}
		}
	}
	return false
}

// matchGroupNegated is `!(list)`: a prefix that none of the alternatives matches, then the
// rest of the pattern.
//
// `!(x)` on its own is therefore "anything that is not x", which is what `rm !(keep)`
// wants. The empty prefix counts, so `!(a)` matches the empty string -- bash agrees.
func matchGroupNegated(group extendedGroup, rest, value []rune) bool {
	for taken := 0; taken <= len(value); taken++ {
		if matchesAnyAlternative(group.alternatives, value[:taken]) {
			continue
		}
		if matchExtendedPattern(rest, value[taken:]) {
			return true
		}
	}
	return false
}

func matchesAnyAlternative(alternatives [][]rune, value []rune) bool {
	for _, alternative := range alternatives {
		if matchExtendedPattern(alternative, value) {
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
	return strings.IndexByte(extendedOpeners, line[index-1]) >= 0
}

// skipBalancedParens returns the index just past the `)` that closes the `(` at index.
func skipBalancedParens(line string, index int) int {
	depth := 0
	for scan := index; scan < len(line); scan++ {
		switch line[scan] {
		case '\\':
			scan++
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return scan + 1
			}
		}
	}
	return len(line)
}

// extendedGroupText is the whole `X(...)` starting at index, for the scans that copy it
// through rather than only stepping over it.
func extendedGroupText(line string, index int) (string, bool) {
	if !extendedGroupOpensAt(line, index) {
		return "", false
	}
	return line[index:skipBalancedParens(line, index)], true
}
