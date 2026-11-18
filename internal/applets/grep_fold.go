package applets

import (
	"bytes"
	"regexp"
	"unicode"
)

// grepMatcher is how grep asks whether a line is selected: its regexp, or, for -i with one plain
// ASCII pattern, a search for the pattern lowered in the line with its ASCII lowered. Go's regexp
// cannot use a literal prefix under (?i), so `grep -ci error` of 92 MB took 1.74 s, where
// busybox's takes 1.24.
//
// The search answers what (?i) does. A pattern's letter whose case folds outside ASCII -- k to
// the Kelvin sign U+212A, s to the long s U+017F -- could match a non-ASCII character, so a line
// holding any non-ASCII byte goes to the regexp when the pattern has such a letter. No other
// non-ASCII byte can be part of a match, and lowering leaves it as it is.
type grepMatcher struct {
	expr *regexp.Regexp
	// folded is the pattern lowered, or nil for the regexp alone.
	folded []byte
	// foldsOut is whether a letter of the pattern folds outside ASCII.
	foldsOut bool
	lowered  []byte
}

func newGrepMatcher(flags grepFlags, expr *regexp.Regexp) *grepMatcher {
	matcher := &grepMatcher{expr: expr}
	if !flags.ignoreCase || len(flags.patterns) != 1 || flags.wordMatch || flags.lineMatch {
		return matcher
	}
	pattern := flags.patterns[0]
	if pattern == "" || !flags.fixedString && regexp.QuoteMeta(pattern) != pattern {
		return matcher
	}
	for index := range len(pattern) {
		if pattern[index] >= 0x80 {
			return matcher
		}
		for folded := unicode.SimpleFold(rune(pattern[index])); folded != rune(pattern[index]); folded = unicode.SimpleFold(folded) {
			if folded >= 0x80 {
				matcher.foldsOut = true
			}
		}
	}
	matcher.folded = asciiLower(nil, []byte(pattern))
	return matcher
}

// match is whether line is selected.
func (m *grepMatcher) match(line []byte) bool {
	if m.folded == nil || m.foldsOut && !isASCII(line) {
		return m.expr.Match(line)
	}
	m.lowered = asciiLower(m.lowered[:0], line)
	return bytes.Contains(m.lowered, m.folded)
}

// asciiLower appends text to into with its ASCII capitals lowered.
func asciiLower(into, text []byte) []byte {
	for _, c := range text {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		into = append(into, c)
	}
	return into
}

func isASCII(text []byte) bool {
	for _, c := range text {
		if c >= 0x80 {
			return false
		}
	}
	return true
}
