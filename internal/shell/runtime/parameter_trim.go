package runtime

import "unicode/utf8"

// trimParameter is the #, ##, % and %% family: strip the shortest or longest
// matching prefix or suffix, where "matching" is the pattern language of 2.13.1
// rather than a literal comparison.
func trimParameter(operator, value, pattern string) string {
	switch operator {
	case "#":
		return trimPatternPrefix(value, pattern, false)
	case "##":
		return trimPatternPrefix(value, pattern, true)
	case "%":
		return trimPatternSuffix(value, pattern, false)
	default:
		return trimPatternSuffix(value, pattern, true)
	}
}

func trimPatternPrefix(value, pattern string, longest bool) string {
	best, characters := -1, cutsCharacters(value, pattern)
	for end := 0; end <= len(value); end++ {
		if !isCut(value, end, characters) || !matchShellPattern(pattern, value[:end]) {
			continue
		}
		best = end
		if !longest {
			break
		}
	}
	if best < 0 {
		return value
	}
	return value[best:]
}

func trimPatternSuffix(value, pattern string, longest bool) string {
	best, characters := -1, cutsCharacters(value, pattern)
	for start := len(value); start >= 0; start-- {
		if !isCut(value, start, characters) || !matchShellPattern(pattern, value[start:]) {
			continue
		}
		best = start
		if !longest {
			break
		}
	}
	if best < 0 {
		return value
	}
	return value[:best]
}

// cutsCharacters says whether a match in value begins and ends only between characters: when
// value and pattern are both UTF-8, as bash matches characters in a UTF-8 locale. nemosh
// matches runes everywhere else -- `${#v}`, `${v:1}`, `case` -- but the trim and replacement
// scans stepped a byte at a time, and a character cut in two read as U+FFFD, which `?` matches:
// `${v#?}` over `μ-` was `\xbc-`. busybox-w32 counts bytes throughout, `${#v}` there too. A
// value or a pattern that is not UTF-8 is still cut at any byte, as bash goes back to bytes
// for one (remove_pattern in subst.c).
func cutsCharacters(value, pattern string) bool {
	return utf8.ValidString(value) && utf8.ValidString(pattern)
}

// isCut is whether a match may begin or end at index in value.
func isCut(value string, index int, characters bool) bool {
	return !characters || index == len(value) || utf8.RuneStart(value[index])
}
