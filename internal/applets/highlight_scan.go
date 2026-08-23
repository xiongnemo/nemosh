package applets

import (
	"regexp"
	"unicode/utf8"
)

// The scanner: what happens at one position in one line.
//
// Split from highlight.go for the line ceiling, and the seam is the state -- inside a
// region nothing is looked for except the way out, and outside one everything is. That
// asymmetry is the whole of why `// not a comment` inside a string stays a string, and
// why a keyword inside a comment is not a keyword.

// scanInsideRegion consumes from position to wherever the innermost open region ends,
// or to the end of the line. It answers the span, the next position, and the stack that
// remains.
func scanInsideRegion(syntax *highlightSyntax, line string, position int, open []int) (highlightSpan, int, []int) {
	region := &syntax.regions[open[len(open)-1]]
	cursor := position
	for cursor < len(line) {
		// skip first, always. A backslash before a quote hides the quote, so the
		// escape has to be consumed before the end is looked for -- otherwise
		// `"a\""` closes at the middle quote and the rest of the line reads as code.
		if match := matchAt(region.skip, line, cursor); match > 0 {
			cursor += match
			continue
		}
		// A nested opener, for the languages that allow one.
		if region.nested {
			if match := matchAt(region.start, line, cursor); match > 0 {
				return highlightSpan{start: position, end: cursor + match, group: region.group},
					cursor + match, append(open, open[len(open)-1])
			}
		}
		if match := matchAt(region.end, line, cursor); match > 0 {
			return highlightSpan{start: position, end: cursor + match, group: region.group},
				cursor + match, open[:len(open)-1]
		}
		cursor += runeLengthAt(line, cursor)
	}
	// Unclosed: the region owns the rest of the line and stays open for the next one.
	return highlightSpan{start: position, end: len(line), group: region.group}, len(line), open
}

// scanOutsideRegion looks for something beginning exactly at this position: a region
// start, or a pattern. It answers the span, the next position, and the index of a
// region that was opened, or -1.
//
// Regions are tried before patterns, which matters at every comment introducer: `//`
// would otherwise be claimed by an operator pattern one character at a time and the
// comment would never open.
func scanOutsideRegion(syntax *highlightSyntax, line string, position int) (highlightSpan, int, int) {
	for index := range syntax.regions {
		region := &syntax.regions[index]
		if match := matchAt(region.start, line, position); match > 0 {
			return highlightSpan{start: position, end: position + match, group: region.group},
				position + match, index
		}
	}
	for _, pattern := range syntax.patterns {
		if pattern.wordOnly && !atWordBoundary(line, position) {
			continue
		}
		if match := matchAt(pattern.match, line, position); match > 0 {
			return highlightSpan{start: position, end: position + match, group: pattern.group},
				position + match, -1
		}
	}
	return highlightSpan{}, position, -1
}

// matchAt reports how many bytes the expression matches *starting exactly at* this
// position, or zero.
//
// The anchoring is the subtle part, and the reason this is a function rather than a
// bare FindStringIndex. Every expression in the tables begins with `^`, and `^` matches
// the start of the string it is given -- so the string given must begin at the position
// being asked about. Handing it the whole line and accepting a match "at or after"
// would let a keyword three characters away claim this position.
//
// The `loc[0] != 0` check is belt and braces: a table entry that forgets its `^` then
// matches nothing rather than matching in the wrong place, which is the failure that
// would be hardest to see.
func matchAt(expression *regexp.Regexp, line string, position int) int {
	if expression == nil {
		return 0
	}
	loc := expression.FindStringIndex(line[position:])
	if loc == nil || loc[0] != 0 {
		return 0
	}
	return loc[1]
}

// atWordBoundary reports whether position begins a word, by looking at the byte before
// it.
//
// This is what a leading `\b` cannot do here. Slicing the line means the expression
// sees no preceding character, so `\b` always succeeds at offset zero of the slice --
// which would make `int` match inside `print`. The tables say `wordOnly: true` instead
// and this answers it.
func atWordBoundary(line string, position int) bool {
	if position == 0 {
		return true
	}
	previous, _ := utf8.DecodeLastRuneInString(line[:position])
	return !isWordRune(previous)
}

func isWordRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	}
	return r == '_'
}

func decodeRuneAt(line string, position int) (rune, int) {
	return utf8.DecodeRuneInString(line[position:])
}
