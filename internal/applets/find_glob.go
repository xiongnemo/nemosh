package applets

import (
	"fmt"
	"regexp"
	"strings"
)

// -path and -ipath match the whole path, and their wildcards cross separators
// where -name's cannot.
//
// That difference is not a detail: GNU find and busybox both call fnmatch
// *without* FNM_PATHNAME for -path, so `-path "./sub*"` matches `./sub/b.txt`.
// Go's path.Match hard-codes the opposite -- its `*` never matches a `/` -- so
// -name can use it and -path cannot. Measured against busybox-w32 on
// 2026-08-22: `find . -path "./sub*"` answers `./sub`, `./sub/c.txt`.
//
// The translation is to a regexp rather than a hand-written matcher because the
// bracket expressions are the fiddly part and regexp already has them, and
// because compiling at parse time means a bad pattern is refused before the
// walk starts rather than silently matching nothing.
func compileFindPathPattern(pattern string) (*regexp.Regexp, error) {
	var builder strings.Builder
	builder.WriteString("^")
	for index := 0; index < len(pattern); index++ {
		switch character := pattern[index]; character {
		case '*':
			builder.WriteString("(?s:.*)")
		case '?':
			builder.WriteString("(?s:.)")
		case '[':
			class, width, err := findPatternClass(pattern[index:])
			if err != nil {
				return nil, err
			}
			builder.WriteString(class)
			index += width - 1
		default:
			builder.WriteString(regexp.QuoteMeta(string(character)))
		}
	}
	builder.WriteString("$")
	return regexp.Compile(builder.String())
}

// findPatternClass copies one bracket expression across, reporting how much of
// the pattern it consumed. A `!` negation becomes regexp's `^`, and an
// unterminated bracket is an error rather than a literal, which is what makes
// `-path "[abc"` a refusal instead of a pattern that matches nothing.
func findPatternClass(pattern string) (string, int, error) {
	var builder strings.Builder
	builder.WriteString("[")
	index := 1
	if index < len(pattern) && (pattern[index] == '!' || pattern[index] == '^') {
		builder.WriteString("^")
		index++
	}
	// A `]` in the first position is a literal, which is the shell's rule and
	// regexp's too once it is escaped.
	if index < len(pattern) && pattern[index] == ']' {
		builder.WriteString(`\]`)
		index++
	}
	for ; index < len(pattern); index++ {
		switch character := pattern[index]; character {
		case ']':
			builder.WriteString("]")
			return builder.String(), index + 1, nil
		case '\\', '^':
			builder.WriteString(`\` + string(character))
		default:
			builder.WriteString(string(character))
		}
	}
	return "", 0, fmt.Errorf("unterminated %q in pattern", "[")
}
