package runtime

// matchShellPattern reports whether value matches the POSIX pattern of 2.13.1:
// `*` for any string including the empty one, `?` for any single character,
// `[...]` for a bracket expression with ranges and a leading `!` or `^` for
// negation, and a backslash to quote the next character.
//
// Deliberately not path.Match: that refuses to let `*` cross a `/`, which is
// right for a pathname and wrong here, where the pattern is matched against a
// whole word. `case /a/b/c in /a/*)` has to match, and busybox ash reaches
// fnmatch without FNM_PATHNAME for exactly that reason (shell/ash.c casematch).
//
// Runes rather than bytes, so `?` matches one character of a non-ASCII word
// instead of one byte of its UTF-8 encoding.
func matchShellPattern(pattern, value string) bool {
	return matchRunePattern([]rune(pattern), []rune(value))
}

func matchRunePattern(pattern, value []rune) bool {
	patternIndex, valueIndex := 0, 0
	starPattern, starValue := -1, 0
	for valueIndex < len(value) {
		if patternIndex < len(pattern) && pattern[patternIndex] == '*' {
			starPattern, starValue = patternIndex, valueIndex
			patternIndex++
			continue
		}
		if patternIndex < len(pattern) {
			if next, ok := matchOneRune(pattern, patternIndex, value[valueIndex]); ok {
				patternIndex, valueIndex = next, valueIndex+1
				continue
			}
		}
		// The last `*` is the only place with anything left to try: give it one
		// more character of the value and resume from just after it.
		if starPattern < 0 {
			return false
		}
		starValue++
		patternIndex, valueIndex = starPattern+1, starValue
	}
	for patternIndex < len(pattern) && pattern[patternIndex] == '*' {
		patternIndex++
	}
	return patternIndex == len(pattern)
}

// matchOneRune consumes the single pattern element at index and reports where
// the pattern continues. A `[` with no closing `]` is an ordinary character,
// which is what POSIX 2.13.1 says an unmatched bracket becomes.
func matchOneRune(pattern []rune, index int, char rune) (int, bool) {
	switch pattern[index] {
	case '?':
		return index + 1, true
	case '[':
		if end, ok := bracketEnd(pattern, index); ok {
			return end, bracketMatches(pattern[index+1:end-1], char)
		}
		return index + 1, char == '['
	case '\\':
		if index+1 < len(pattern) {
			return index + 2, pattern[index+1] == char
		}
		return index + 1, char == '\\'
	default:
		return index + 1, pattern[index] == char
	}
}

// bracketEnd reports the index just past the `]` that closes the bracket
// expression opened at start. A `!` or `^` right after the `[` is the negation
// marker and a `]` right after that is data, so neither closes it.
func bracketEnd(pattern []rune, start int) (int, bool) {
	index := start + 1
	if index < len(pattern) && (pattern[index] == '!' || pattern[index] == '^') {
		index++
	}
	if index < len(pattern) && pattern[index] == ']' {
		index++
	}
	for ; index < len(pattern); index++ {
		if pattern[index] == ']' {
			return index + 1, true
		}
	}
	return 0, false
}

func bracketMatches(spec []rune, char rune) bool {
	negated := false
	if len(spec) > 0 && (spec[0] == '!' || spec[0] == '^') {
		negated, spec = true, spec[1:]
	}
	matched := false
	for index := 0; index < len(spec); index++ {
		if index+2 < len(spec) && spec[index+1] == '-' {
			if char >= spec[index] && char <= spec[index+2] {
				matched = true
			}
			index += 2
			continue
		}
		if spec[index] == char {
			matched = true
		}
	}
	return matched != negated
}
