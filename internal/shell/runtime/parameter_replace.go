package runtime

import "strings"

// parameterReplace is `${name/pattern/replacement}` and its three variants: `//`
// for every match, `/#` anchored at the start, `/%` anchored at the end.
//
// The pattern is a shell pattern rather than a regular expression, and the match is
// greedy: measured, `${x/b*/-}` over `a1b2c` gives `a1-`, so `b*` took everything
// it could. A missing replacement deletes -- `${x//X}` is `${x//X/}`.
//
// fold is `shopt -s nocasematch`: the match is looked for in an ASCII-folded copy, and
// the replacement cut out of the original, whose offsets are the same (pattern_nocase.go).
func parameterReplace(value, operator, spec string, fold bool) string {
	pattern, replacement := splitReplacementSpec(spec)
	if pattern == "" {
		// An empty pattern replaces nothing, except anchored: `${x/#/p-}` prefixes and
		// `${x/%/-s}` appends, as bash has them, and `${files[@]/#/--include=}` is how a
		// script puts a prefix on every element. Both returned the value unchanged.
		// busybox-w32 has neither anchor.
		switch operator {
		case "/#":
			return replacement + value
		case "/%":
			return value + replacement
		}
		return value
	}
	subject := value
	if fold {
		subject, pattern = foldASCII(value), foldASCII(pattern)
	}
	characters := cutsCharacters(subject, pattern)
	if characters && !strings.ContainsAny(pattern, `*?[\`) && !hasExtendedPattern(pattern) {
		return replaceLiteral(value, subject, pattern, replacement, operator)
	}
	switch operator {
	case "/#":
		if width, ok := longestPatternMatchAt(subject, pattern, 0, characters); ok {
			return replacement + value[width:]
		}
		return value
	case "/%":
		for start := 0; start <= len(subject); start++ {
			if isCut(subject, start, characters) && matchShellPattern(pattern, subject[start:]) {
				return value[:start] + replacement
			}
		}
		return value
	}
	return replaceMatches(value, subject, pattern, replacement, operator == "//", characters)
}

// replaceLiteral is parameterReplace of a pattern that is its own text, a search for it rather
// than the scan below, which tried the pattern from every place to every place after it, making
// both into runes each time: `${v//0/1}` of 3,000 characters took 6.3 s and 224 MiB of commit,
// busybox's 41 ms. The longest match of a literal is itself, and with value and pattern both
// UTF-8 the text is found only where a character begins and ends, so the answers are the scan's.
func replaceLiteral(value, subject, pattern, replacement, operator string) string {
	switch operator {
	case "/#":
		if strings.HasPrefix(subject, pattern) {
			return replacement + value[len(pattern):]
		}
		return value
	case "/%":
		if strings.HasSuffix(subject, pattern) {
			return value[:len(value)-len(pattern)] + replacement
		}
		return value
	}
	at := strings.Index(subject, pattern)
	if at < 0 {
		return value
	}
	var out strings.Builder
	out.Grow(len(value))
	rest := 0
	for at >= 0 {
		out.WriteString(value[rest : rest+at])
		out.WriteString(replacement)
		rest += at + len(pattern)
		if operator != "//" {
			break
		}
		at = strings.Index(subject[rest:], pattern)
	}
	out.WriteString(value[rest:])
	return out.String()
}

// replaceMatches finds matches in subject and writes value around them: the two are the
// same string, or one is the other folded, with the same offsets. A match begins and ends
// only between characters when characters is set (cutsCharacters).
func replaceMatches(value, subject, pattern, replacement string, all, characters bool) string {
	var out strings.Builder
	for index := 0; index <= len(value); {
		if !isCut(subject, index, characters) {
			out.WriteByte(value[index])
			index++
			continue
		}
		width, ok := longestPatternMatchAt(subject, pattern, index, characters)
		if !ok {
			if index == len(value) {
				break
			}
			out.WriteByte(value[index])
			index++
			continue
		}
		out.WriteString(replacement)
		if width == 0 {
			// An empty match would otherwise never advance. bash replaces once at
			// each position in that case rather than spinning.
			if index == len(value) {
				break
			}
			out.WriteByte(value[index])
			index++
		} else {
			index += width
			// A match that reached the end leaves nothing: the empty tail is no second match,
			// so `${v//*/-}` is one -, as in bash. It was two; busybox loops for ever.
			if all && index == len(value) {
				break
			}
		}
		if !all {
			out.WriteString(value[index:])
			return out.String()
		}
	}
	return out.String()
}

// longestPatternMatchAt reports how many bytes of value, starting at index, the
// pattern matches, taking the longest.
//
// Built on matchShellPattern rather than on a new matcher, because that one is
// already the shell's definition of a pattern and is already fuzzed. The cost is a
// scan per position, which is nothing at the sizes a variable holds.
func longestPatternMatchAt(value, pattern string, index int, characters bool) (int, bool) {
	for end := len(value); end >= index; end-- {
		if isCut(value, end, characters) && matchShellPattern(pattern, value[index:end]) {
			return end - index, true
		}
	}
	return 0, false
}

// splitReplacementSpec cuts `pattern/replacement` at the first unescaped slash. A
// slash can be part of the pattern when escaped, which is what lets a path be
// matched.
func splitReplacementSpec(spec string) (string, string) {
	var pattern strings.Builder
	for index := 0; index < len(spec); index++ {
		if spec[index] == '\\' && index+1 < len(spec) && spec[index+1] == '/' {
			pattern.WriteByte('/')
			index++
			continue
		}
		if spec[index] == '/' {
			return pattern.String(), spec[index+1:]
		}
		pattern.WriteByte(spec[index])
	}
	return pattern.String(), ""
}
