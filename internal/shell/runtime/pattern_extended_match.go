package runtime

import "strings"

// The extended operators need a real search: `+(a|ab)` against `aab` has to try both
// alternatives at both positions, and a `*` before a group every split of what is left. The
// search tries every split, every alternative and every count, and remembers each answer:
// whether a stretch of the pattern matches a stretch of the value, and whether a group's
// repetitions from a position do, are each worked out once.
//
// It forgot them, and was exponential in nested operators: `**(*)1` against thirty
// characters, and `+(+(0|p))1` against a run of them, never came back -- bash's own matcher
// never does either. Remembered, the work is bounded by the stretches there are to ask
// about, a few for each part of the pattern for each pair of positions in the value.
// FuzzMatchShellPattern found the first.

// extendedMatch is one search: the pattern and value as runes, and the answers so far.
type extendedMatch struct {
	pattern, value []rune
	memo           map[extendedQuestion]bool
}

// extendedQuestion is one answer's key: pattern[from:to] against value[start:end], or with
// group set, the group there repeated at least and at most times and then pattern[rest:to].
type extendedQuestion struct {
	group          bool
	from, to, rest int
	start, end     int
	least, most    int
}

// extendedGroup is a parsed `X(a|b|c)`: the operator, each alternative's bounds in the
// pattern, and where the pattern continues, just past the closing parenthesis.
type extendedGroup struct {
	operator     rune
	alternatives [][2]int
	end          int
}

// matchExtendedPattern is matchShellPattern for a pattern that uses the operators.
func matchExtendedPattern(pattern, value []rune) bool {
	search := &extendedMatch{pattern: pattern, value: value, memo: map[extendedQuestion]bool{}}
	return search.match(0, len(pattern), 0, len(value))
}

// match answers whether pattern[from:to] matches the whole of value[start:end].
func (m *extendedMatch) match(from, to, start, end int) bool {
	question := extendedQuestion{from: from, to: to, start: start, end: end}
	if answer, known := m.memo[question]; known {
		return answer
	}
	answer := m.matchFresh(from, to, start, end)
	m.memo[question] = answer
	return answer
}

func (m *extendedMatch) matchFresh(from, to, start, end int) bool {
	if from == to {
		return start == end
	}
	pattern := m.pattern[:to]
	if group, ok := extendedGroupAt(pattern, from); ok {
		if group.operator == '!' {
			return m.negated(group, to, start, end)
		}
		least, most := 1, 1
		switch group.operator {
		case '?':
			least = 0
		case '*':
			least, most = 0, -1
		case '+':
			most = -1
		}
		return m.repeated(group, from, to, start, end, least, most)
	}
	if pattern[from] == '*' {
		// Every split, shortest first.
		for taken := start; taken <= end; taken++ {
			if m.match(from+1, to, taken, end) {
				return true
			}
		}
		return false
	}
	if start == end {
		return false
	}
	next, ok := matchOneRune(pattern, from, m.value[start])
	return ok && m.match(next, to, start+1, end)
}

// repeated matches the group at from between least and most times, where -1 is no limit,
// from start, and then the rest of the pattern against what is left. A search over prefixes
// rather than a greedy loop, which gets `+(a|ab)` against `aab` wrong: it takes a, then a,
// and has b left over and no way back.
func (m *extendedMatch) repeated(group extendedGroup, from, to, start, end, least, most int) bool {
	least = max(least, 0)
	question := extendedQuestion{group: true, from: from, to: to, rest: group.end, start: start, end: end, least: least, most: most}
	if answer, known := m.memo[question]; known {
		return answer
	}
	answer := m.repeatedFresh(group, from, to, start, end, least, most)
	m.memo[question] = answer
	return answer
}

func (m *extendedMatch) repeatedFresh(group extendedGroup, from, to, start, end, least, most int) bool {
	if least == 0 && m.match(group.end, to, start, end) {
		return true
	}
	if most == 0 {
		return false
	}
	remaining := most
	if remaining > 0 {
		remaining--
	}
	// An empty prefix only while the group still owes a match, or `*(a)` would ask the
	// same question of the same position for ever -- and then it counts, or `@()` and the
	// empty arm of `@(a||b)` could never match the empty string they match in bash.
	first := 1
	if least > 0 {
		first = 0
	}
	for _, alternative := range group.alternatives {
		for taken := start + first; taken <= end; taken++ {
			if m.match(alternative[0], alternative[1], start, taken) &&
				m.repeated(group, from, to, taken, end, least-1, remaining) {
				return true
			}
		}
	}
	return false
}

// negated is `!(list)`: a prefix none of the alternatives matches, then the rest of the
// pattern. `!(x)` alone is anything that is not x, which is what `rm !(keep)` wants, and the
// empty prefix counts, so `!(a)` matches the empty string, as in bash.
func (m *extendedMatch) negated(group extendedGroup, to, start, end int) bool {
	for taken := start; taken <= end; taken++ {
		matched := false
		for _, alternative := range group.alternatives {
			if m.match(alternative[0], alternative[1], start, taken) {
				matched = true
				break
			}
		}
		if !matched && m.match(group.end, to, taken, end) {
			return true
		}
	}
	return false
}

// extendedGroupAt reads a group at index, reporting whether one is there. One with no
// closing parenthesis is no group, and its characters are ordinary, as POSIX says of an
// unmatched bracket.
func extendedGroupAt(pattern []rune, index int) (extendedGroup, bool) {
	if index+1 >= len(pattern) || pattern[index+1] != '(' || !strings.ContainsRune(extendedOpeners, pattern[index]) {
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
			if depth--; depth == 0 {
				return extendedGroup{operator: pattern[index], alternatives: splitAlternatives(pattern, index+2, scan), end: scan + 1}, true
			}
		}
	}
	return extendedGroup{}, false
}

// splitAlternatives cuts pattern[from:to] on its top-level `|`, so `@(a|b(c|d))` has two,
// and answers each one's bounds.
func splitAlternatives(pattern []rune, from, to int) [][2]int {
	var alternatives [][2]int
	depth, start := 0, from
	for index := from; index < to; index++ {
		switch pattern[index] {
		case '\\':
			index++
		case '(':
			depth++
		case ')':
			depth--
		case '|':
			if depth == 0 {
				alternatives = append(alternatives, [2]int{start, index})
				start = index + 1
			}
		}
	}
	return append(alternatives, [2]int{start, to})
}
