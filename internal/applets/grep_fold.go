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
//
// -w of one plain pattern is a search too, for the pattern with no ASCII word character either
// side of it, which is what the regexp -w makes, `(?:\A|\W)(word)(?:\z|\W)`, answers: its leading
// alternation leaves Go no literal prefix either, and `grep -cw err` of 88 MB took 2.99 s, where
// busybox's takes 0.94. A byte of a character outside ASCII is no word character, as \W is that
// character.
type grepMatcher struct {
	expr *regexp.Regexp
	// folded is the pattern lowered, or nil for the regexp alone.
	folded []byte
	// foldsOut is whether a letter of the pattern folds outside ASCII.
	foldsOut bool
	lowered  []byte
	// word is the pattern searched for whole under -w, as it is or, under -i, folded; nil for
	// the regexp.
	word []byte
}

func newGrepMatcher(flags grepFlags, expr *regexp.Regexp) *grepMatcher {
	matcher := &grepMatcher{expr: expr}
	if !flags.ignoreCase && !flags.wordMatch || len(flags.patterns) != 1 || flags.lineMatch {
		return matcher
	}
	pattern := flags.patterns[0]
	if pattern == "" || !flags.fixedString && regexp.QuoteMeta(pattern) != pattern {
		return matcher
	}
	if flags.ignoreCase {
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
	}
	if flags.wordMatch {
		matcher.word = []byte(pattern)
		if matcher.folded != nil {
			matcher.word = matcher.folded
		}
	}
	return matcher
}

// match is whether line is selected.
func (m *grepMatcher) match(line []byte) bool {
	if m.folded == nil && m.word == nil || m.foldsOut && !isASCII(line) {
		return m.expr.Match(line)
	}
	text := line
	if m.folded != nil {
		m.lowered = asciiLower(m.lowered[:0], line)
		text = m.lowered
	}
	if m.word != nil {
		return containsWord(text, m.word)
	}
	return bytes.Contains(text, m.folded)
}

// containsWord is whether word is somewhere in text with no ASCII word character on either side.
func containsWord(text, word []byte) bool {
	for start := 0; start <= len(text)-len(word); {
		at := bytes.Index(text[start:], word)
		if at < 0 {
			return false
		}
		at += start
		end := at + len(word)
		if (at == 0 || !isWordByte(text[at-1])) && (end == len(text) || !isWordByte(text[end])) {
			return true
		}
		start = at + 1
	}
	return false
}

func isWordByte(c byte) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
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
