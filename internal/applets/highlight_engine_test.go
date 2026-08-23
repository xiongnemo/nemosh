package applets

import (
	"regexp"
	"strings"
	"testing"
)

// The engine, against a tiny made-up language rather than a real one.
//
// A fixture syntax is what makes these tests about the *engine*: a failure here is a
// scanning bug and not a mistake in a keyword list. The real languages are checked
// separately, against real source files.

// testSyntax is C-like enough to exercise every branch: a line comment, a nestable
// block comment, a string with escapes, one keyword and one number rule.
func testSyntax() *highlightSyntax {
	return &highlightSyntax{
		name: "fixture",
		regions: []highlightRegion{
			{start: regexp.MustCompile(`^/\*`), end: regexp.MustCompile(`^\*/`), group: groupComment},
			{start: regexp.MustCompile(`^\{-`), end: regexp.MustCompile(`^-\}`), nested: true, group: groupComment},
			{
				start: regexp.MustCompile(`^"`),
				end:   regexp.MustCompile(`^"`),
				skip:  regexp.MustCompile(`^\\.`),
				group: groupString,
			},
		},
		patterns: []highlightPattern{
			// A line comment is a *pattern* to the end of the line and not a region.
			// As a region with an end of `^$` it never closed -- the scan stops at the
			// end of the line before an empty match can happen -- so the comment
			// carried into every following line. Measured: `x // c` followed by
			// `func y` coloured the second line entirely as comment.
			{match: regexp.MustCompile(`^//.*`), group: groupComment},
			{match: regexp.MustCompile(`^(?:func|return|int)\b`), group: groupKeyword, wordOnly: true},
			{match: regexp.MustCompile(`^[0-9]+`), group: groupNumber, wordOnly: true},
		},
	}
}

// render draws the groups under the text, which makes a failure readable: the two
// lines line up and the wrong character is directly under its wrong colour.
func render(line string, spans []highlightSpan) string {
	marks := []byte(strings.Repeat(".", len(line)))
	letters := map[highlightGroup]byte{
		groupComment: 'c', groupString: 's', groupKeyword: 'k',
		groupNumber: 'n', groupType: 't', groupSymbol: 'y',
	}
	for _, span := range spans {
		for index := span.start; index < span.end && index < len(marks); index++ {
			marks[index] = letters[span.group]
		}
	}
	return string(marks)
}

func TestHighlightLine_patternsAndRegions(t *testing.T) {
	syntax := testSyntax()
	for _, test := range []struct{ line, want string }{
		// A keyword, and the same letters inside a longer word, which must not match.
		{line: `func f() {}`, want: `kkkk.......`},
		{line: `printer int`, want: `........kkk`},
		{line: `myfunc`, want: `......`},
		// A number, and digits inside an identifier, which are not a number.
		{line: `x = 42`, want: `....nn`},
		{line: `a1b2`, want: `....`},
		// A line comment owns the rest of the line, keywords inside it included.
		{line: `x // func 42`, want: `..cccccccccc`},
		// A string owns its contents, and the escape does not end it.
		{line: `"func"`, want: `ssssss`},
		// Six bytes of string, not seven: the backslash and the quote it hides are
		// two bytes and one of them is not a delimiter. Counted, after the first
		// draft of this expectation was one character too long.
		{line: `"a\"b" func`, want: `ssssss.kkkk`},
		// A comment introducer inside a string is not a comment.
		{line: `"// not"`, want: `ssssssss`},
		// And a string quote inside a comment is not a string.
		{line: `// "x`, want: `ccccc`},
		// Two regions on one line, closing in order.
		{line: `/* a */ func`, want: `ccccccc.kkkk`},
		// An empty line is no spans and no crash.
		{line: ``, want: ``},
	} {
		t.Run(test.line, func(t *testing.T) {
			spans, _ := highlightLine(syntax, test.line, nil)
			if got := render(test.line, spans); got != test.want {
				t.Fatalf("\n line %q\n  got %s\n want %s", test.line, got, test.want)
			}
		})
	}
}

// A region that does not close carries into the next line, which is the whole reason
// the buffer is lexed from the top.
func TestHighlightBuffer_regionsSpanLines(t *testing.T) {
	syntax := testSyntax()
	lines := []string{
		`func a() {`,
		`/* comment`,
		`still func comment`,
		`*/ func b`,
		`func c`,
	}
	want := []string{
		`kkkk......`,
		`cccccccccc`,
		`cccccccccccccccccc`,
		`cc.kkkk..`,
		`kkkk..`,
	}
	spans := highlightBuffer(syntax, lines)
	if len(spans) != len(lines) {
		t.Fatalf("got %d lines of spans for %d lines", len(spans), len(lines))
	}
	for index := range lines {
		if got := render(lines[index], spans[index]); got != want[index] {
			t.Errorf("line %d %q\n  got %s\n want %s", index, lines[index], got, want[index])
		}
	}
}

// Nesting, which Haskell needs and C must not have. The fixture has one region of
// each kind so the difference is visible in one test.
func TestHighlightBuffer_nestedRegionsNeedTwoClosers(t *testing.T) {
	syntax := testSyntax()
	// A nested block comment: the inner closer does not end the outer one.
	nested := []string{`{- outer {- inner -} still -} func`}
	spans := highlightBuffer(syntax, nested)
	if got, want := render(nested[0], spans[0]), `ccccccccccccccccccccccccccccc.kkkk`; got != want {
		t.Errorf("nested comment\n  got %s\n want %s", got, want)
	}
	// A non-nesting one: the first closer ends it, whatever came before.
	flat := []string{`/* outer /* inner */ func`}
	spans = highlightBuffer(syntax, flat)
	if got, want := render(flat[0], spans[0]), `cccccccccccccccccccc.kkkk`; got != want {
		t.Errorf("flat comment\n  got %s\n want %s", got, want)
	}
}

// A multibyte character must never be split by a span boundary, because half a rune
// coloured differently is a rendering bug that only shows on a CJK file -- which on
// this platform is a common file.
func TestHighlightLine_neverSplitsARune(t *testing.T) {
	syntax := testSyntax()
	for _, line := range []string{
		`路径 func`,
		`"路径" func`,
		`// 注释 func`,
		`x = 42 路径`,
		`日本語`,
	} {
		spans, _ := highlightLine(syntax, line, nil)
		for _, span := range spans {
			if !isRuneBoundary(line, span.start) {
				t.Errorf("in %q a span starts at byte %d, which is inside a character", line, span.start)
			}
			if !isRuneBoundary(line, span.end) {
				t.Errorf("in %q a span ends at byte %d, which is inside a character", line, span.end)
			}
		}
	}
}

// Spans never overlap and are in order, which the renderer depends on: an overlapping
// pair would colour the same cell twice and the later one would win silently.
func TestHighlightLine_spansAreOrderedAndDisjoint(t *testing.T) {
	syntax := testSyntax()
	for _, line := range []string{
		`func f() { return 42 } // done`,
		`"a\"b" /* c */ int 7`,
		`{- {- -} -} func`,
		``,
		`                    `,
	} {
		spans, _ := highlightLine(syntax, line, nil)
		previous := 0
		for _, span := range spans {
			if span.start < previous {
				t.Errorf("in %q a span starts at %d after one ended at %d", line, span.start, previous)
			}
			if span.end < span.start {
				t.Errorf("in %q a span ends before it starts: %d..%d", line, span.start, span.end)
			}
			if span.end > len(line) {
				t.Errorf("in %q a span ends at %d past the line's %d bytes", line, span.end, len(line))
			}
			previous = span.end
		}
	}
}

// A nil syntax highlights nothing rather than crashing, which is what a file with no
// recognised extension gets.
func TestHighlightBuffer_nilSyntaxIsNoHighlighting(t *testing.T) {
	if spans := highlightBuffer(nil, []string{"func x", "y"}); spans != nil {
		t.Fatalf("a nil syntax produced %d lines of spans", len(spans))
	}
}

func isRuneBoundary(line string, offset int) bool {
	if offset == 0 || offset == len(line) {
		return true
	}
	if offset < 0 || offset > len(line) {
		return false
	}
	// A continuation byte is 10xxxxxx; anything else starts a character.
	return line[offset]&0xc0 != 0x80
}

// A line comment must not carry into the next line, which is why it is a pattern
// rather than a region. As a region ending at `^$` it never closed: the scan stops at
// the end of the line before an empty-string match can happen, so every following line
// inherited the comment. This is the test that would have caught it.
func TestHighlightBuffer_aLineCommentDoesNotCarry(t *testing.T) {
	syntax := testSyntax()
	lines := []string{`x // comment`, `func y`, `int z`}
	want := []string{`..cccccccccc`, `kkkk..`, `kkk..`}
	spans := highlightBuffer(syntax, lines)
	for index := range lines {
		if got := render(lines[index], spans[index]); got != want[index] {
			t.Errorf("line %d %q\n  got %s\n want %s", index, lines[index], got, want[index])
		}
	}
}

// The other half of the same distinction: an unclosed *block* comment must carry, and
// a closed one must not.
func TestHighlightBuffer_anUnclosedBlockCommentCarries(t *testing.T) {
	syntax := testSyntax()
	lines := []string{`/* open`, `func inside`, `*/ func out`}
	want := []string{`ccccccc`, `ccccccccccc`, `cc.kkkk....`}
	spans := highlightBuffer(syntax, lines)
	for index := range lines {
		if got := render(lines[index], spans[index]); got != want[index] {
			t.Errorf("line %d %q\n  got %s\n want %s", index, lines[index], got, want[index])
		}
	}
}
