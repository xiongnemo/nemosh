package runtime

import (
	"strings"
	"unicode/utf8"
)

// trimParameter is the #, ##, % and %% family: strip the shortest or longest
// matching prefix or suffix, where "matching" is the pattern language of 2.13.1
// rather than a literal comparison.
//
// Each cut point was matched afresh, the pattern and the value converted to characters for each:
// `${p##*/}` of an 80-character path cost 20 µs and 176 allocations. The shapes nearly every trim
// has -- a literal with a `*` before it, after it or both, `*/`, `.*` -- are a search of the
// value now, and any other pattern is converted once a trim. parameter_trim_oracle_test.go holds
// both to the trims as they were written.
func trimParameter(operator, value, pattern string) string {
	prefix, longest := operator[0] == '#', len(operator) == 2
	if cutsCharacters(value, pattern) {
		if cut, ok := literalTrim(value, pattern, prefix, longest); ok {
			if prefix {
				return value[cut:]
			}
			return value[:cut]
		}
		return trimCharacters(value, pattern, prefix, longest)
	}
	if prefix {
		return trimPatternPrefix(value, pattern, longest)
	}
	return trimPatternSuffix(value, pattern, longest)
}

// trimCharacters is a trim of a value and a pattern that are both UTF-8, cut only between
// characters: each converted once, and the cut points tried in the order the answer is the
// first match of -- from the start for the shortest prefix and the longest suffix, from the end
// for the others. Every one was tried, and the last that matched kept.
func trimCharacters(value, pattern string, prefix, longest bool) string {
	compiled, runes := []rune(pattern), []rune(value)
	extended := hasExtendedPattern(pattern)
	ascending := prefix != longest
	count, offset := 0, 0
	if !ascending {
		count, offset = len(runes), len(value)
	}
	for {
		part := runes[count:]
		if prefix {
			part = runes[:count]
		}
		if extended && matchExtendedPattern(compiled, part) || !extended && matchRunePattern(compiled, part) {
			if prefix {
				return value[offset:]
			}
			return value[:offset]
		}
		switch {
		case ascending && count < len(runes):
			offset += utf8.RuneLen(runes[count])
			count++
		case !ascending && count > 0:
			count--
			offset -= utf8.RuneLen(runes[count])
		default:
			return value
		}
	}
}

// literalTrim is a trim whose pattern is a literal with a `*` before it, after it, both or
// neither, answered by a search: where the cut is, the end of the prefix or the start of the
// suffix to remove, and false for any other pattern. Valid UTF-8 has no match of valid UTF-8
// starting or ending inside a character, so the cut is always between two.
func literalTrim(value, pattern string, prefix, longest bool) (int, bool) {
	literal, leading, trailing, ok := literalPattern(pattern)
	if !ok {
		return 0, false
	}
	first, last := strings.Index(value, literal), strings.LastIndex(value, literal)
	if prefix {
		switch {
		case first < 0 || !leading && first != 0:
			return 0, true
		case trailing && longest:
			return len(value), true
		case leading && longest:
			return last + len(literal), true
		}
		return first + len(literal), true
	}
	ending := last >= 0 && last == len(value)-len(literal)
	switch {
	case last < 0 || !trailing && !ending:
		return len(value), true
	case leading && longest:
		return 0, true
	case trailing && longest:
		return first, true
	case trailing:
		return last, true
	}
	return len(value) - len(literal), true
}

// literalPattern is pattern as a literal and whether a `*` stands before it and after it: the
// characters between, a backslash's taken as written, with no other `*`, no `?`, no bracket and
// no extended operator among them.
func literalPattern(pattern string) (literal string, leading, trailing, ok bool) {
	if hasExtendedPattern(pattern) {
		return "", false, false, false
	}
	body := pattern
	if strings.HasPrefix(body, "*") {
		leading, body = true, body[1:]
	}
	escaped := false
	var unescaped strings.Builder
	for index := 0; index < len(body); index++ {
		switch body[index] {
		case '\\':
			if index+1 == len(body) {
				return "", false, false, false
			}
			escaped = true
			unescaped.WriteString(body[:index])
			unescaped.WriteByte(body[index+1])
			body, index = body[index+2:], -1
		case '*':
			if index != len(body)-1 {
				return "", false, false, false
			}
			trailing, body = true, body[:index]
		case '?', '[':
			return "", false, false, false
		}
	}
	if escaped {
		unescaped.WriteString(body)
		return unescaped.String(), leading, trailing, true
	}
	return body, leading, trailing, true
}

// trimPatternPrefix and trimPatternSuffix are the trims where the value or the pattern is not
// UTF-8, which bash cuts at any byte (remove_pattern in subst.c), as these do.
func trimPatternPrefix(value, pattern string, longest bool) string {
	best := -1
	for end := 0; end <= len(value); end++ {
		if !matchShellPattern(pattern, value[:end]) {
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
	best := -1
	for start := len(value); start >= 0; start-- {
		if !matchShellPattern(pattern, value[start:]) {
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
