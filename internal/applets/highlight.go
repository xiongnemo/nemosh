package applets

import "regexp"

// Syntax highlighting for the editor, written here rather than imported.
//
// The decision was measured before it was made. micro's `pkg/highlight` *is*
// importable -- three files, no tcell, MIT -- and inside this binary it costs 283 KiB
// plus a 44 KiB YAML corpus, which the 14 MiB ceiling has room for. What settled it
// was the other ceiling: `gopkg.in/yaml.v2` does **262 allocations in its own package
// init**, taking the total from 2275 to 2545 against a limit of 2750 and leaving 205
// for everything that comes after. This engine's init does none.
//
// Generating Go tables from micro's YAML at build time -- keeping its rules and
// dropping its parser -- is not possible either: `Def.rules` and the `rules`, `region`
// and `pattern` types are all unexported, so a Def can only be built by ParseDef,
// which needs the YAML.
//
// So this is `regexp`, which ten applets already link, over tables written as Go
// literals. The model is micro's, because it is the right one: two kinds of rule.
//
//   - A **pattern** matches within one line: a keyword, a number, an operator.
//   - A **region** runs from a start to an end and need not close on the same line:
//     a block comment, a multi-line string. Regions may nest, which Haskell's
//     `{- {- -} -}` requires and C's `/* /* */` does not, and carry a `skip` for what
//     an escape hides -- without it `"a\""` ends at the middle quote.
//
// The line-spanning part is why a line cannot be highlighted alone. A buffer is lexed
// from the top so each line begins knowing which regions are open, which is what
// `highlightBuffer` is for.

// highlightGroup is what a span of text *is*.
//
// Deliberately few. Six groups tell a reader what they need at a glance; twenty are a
// decision per token that nobody makes consistently across eleven languages.
type highlightGroup uint8

const (
	groupNone highlightGroup = iota
	groupComment
	groupString
	groupKeyword
	groupNumber
	groupType
	groupSymbol
)

// highlightSpan is a run of one group within a line, in **bytes**.
//
// Bytes rather than runes because the editor's cursor arithmetic is in bytes --
// measured, tview's Replace and Select take byte offsets, and counting runes there put
// the cursor in the wrong place on any line holding a multibyte character. One unit
// throughout is what keeps the highlighter and the cursor agreeing.
type highlightSpan struct {
	start int
	end   int
	group highlightGroup
}

// highlightPattern is a single-line rule.
type highlightPattern struct {
	match *regexp.Regexp
	group highlightGroup
	// wordOnly requires the match to begin a word. A leading word-boundary escape
	// cannot express this here: the scanner slices the line, so the expression sees
	// no preceding character and the boundary always succeeds at offset zero --
	// which would make `int` match inside `print`. See atWordBoundary.
	wordOnly bool
}

// highlightRegion may span lines.
type highlightRegion struct {
	start *regexp.Regexp
	end   *regexp.Regexp
	// skip matches what must not be read as an end -- a backslash escape, usually.
	// Checked before end at each position, which is what makes `"a\""` one string.
	skip *regexp.Regexp
	// nested says a start inside the region opens another of the same kind.
	nested bool
	group  highlightGroup
}

// highlightSyntax is one language's rules.
type highlightSyntax struct {
	name string
	// extensions are matched against a lower-cased file name. Longest wins, so
	// `.tar.gz` cannot lose to `.gz`.
	extensions []string
	// filenames are whole names with no useful extension: Makefile, Dockerfile.
	filenames []string
	// regions are tried in order and before patterns, so a comment introducer beats
	// the operator rule that would otherwise claim its first character.
	regions  []highlightRegion
	patterns []highlightPattern
}

// highlightBuffer lexes every line and answers the spans for each.
//
// The whole buffer rather than the visible part, because a line's meaning depends on
// every line above it: a viewport scrolled into the middle of a block comment would
// otherwise be coloured as code. The editor calls this when the text changes, not when
// it scrolls.
func highlightBuffer(syntax *highlightSyntax, lines []string) [][]highlightSpan {
	if syntax == nil {
		return nil
	}
	spans := make([][]highlightSpan, len(lines))
	// open is the stack of region indices still unclosed, carried between lines.
	var open []int
	for index, line := range lines {
		spans[index], open = highlightLine(syntax, line, open)
	}
	return spans
}

// highlightLine lexes one line from a known state, answering its spans and the state
// the next line begins in.
//
// The incoming stack is copied rather than shared: a caller that kept a reference to
// an earlier line's state would otherwise see it mutated, and re-lexing from a
// checkpoint is a thing an editor eventually wants.
func highlightLine(syntax *highlightSyntax, line string, incoming []int) ([]highlightSpan, []int) {
	open := append(make([]int, 0, len(incoming)+1), incoming...)
	spans := make([]highlightSpan, 0, 8)
	position := 0
	for position < len(line) {
		if len(open) > 0 {
			span, next, stillOpen := scanInsideRegion(syntax, line, position, open)
			spans = append(spans, span)
			open, position = stillOpen, next
			continue
		}
		span, next, opened := scanOutsideRegion(syntax, line, position)
		if next <= position {
			// Nothing matched here. Advance one character, not one byte, so a
			// multibyte character is never split -- a span boundary inside a rune
			// would colour half of it.
			position += runeLengthAt(line, position)
			continue
		}
		if span.group != groupNone {
			spans = append(spans, span)
		}
		if opened >= 0 {
			open = append(open, opened)
		}
		position = next
	}
	return spans, open
}

// runeLengthAt is how many bytes the character at this offset occupies.
//
// utf8.DecodeRuneInString answers 1 for invalid input, which is what makes this safe
// on a file that is not text: the scan advances rather than looping.
func runeLengthAt(line string, position int) int {
	if position >= len(line) {
		return 1
	}
	_, size := decodeRuneAt(line, position)
	if size < 1 {
		return 1
	}
	return size
}
