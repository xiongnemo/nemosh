package applets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Whole files, for the two languages whose traps only appear across lines.
//
// The line-by-line tests cover each rule; this covers the thing they cannot -- a real
// file, lexed from the top, where a mistake in a multi-line region shows up as every
// subsequent line being the wrong colour. Both fixtures are in testdata and were
// written before the tables, which is why the tables have the awkward cases in them at
// all.

func loadHighlightSample(t *testing.T, name string) ([]string, [][]highlightSpan) {
	t.Helper()
	path := filepath.Join("testdata", "highlight", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	syntax := highlightSyntaxFor(name)
	if syntax == nil {
		t.Fatalf("%s matched no syntax table", name)
	}
	return lines, highlightBuffer(syntax, lines)
}

// groupsOn answers the set of groups present on a line, which is the right granularity
// for "is this line code or comment" without pinning every column.
func groupsOn(spans []highlightSpan) map[highlightGroup]bool {
	present := map[highlightGroup]bool{}
	for _, span := range spans {
		present[span.group] = true
	}
	return present
}

// Haskell: the nested block comment is the case the engine's `nested` flag exists for,
// and the failure it guards against is silent -- the inner `-}` closing the outer
// comment would leave the rest of the file coloured as code, which looks *fine*.
func TestHighlightFile_haskellNestedCommentEndsWhereItShould(t *testing.T) {
	lines, spans := loadHighlightSample(t, "sample.hs")

	nested := -1
	for index, line := range lines {
		if strings.HasPrefix(line, "{- outer") {
			nested = index
		}
	}
	if nested < 0 {
		t.Fatal("the fixture no longer holds the nested comment this test is about")
	}
	// The whole nested line is comment, inner closer included.
	if got := groupsOn(spans[nested]); len(got) != 1 || !got[groupComment] {
		t.Fatalf("the nested comment line is not all comment:\n  %s\n  %s",
			lines[nested], render(lines[nested], spans[nested]))
	}
	// And the line after it is code again, which is what says the comment closed on
	// the *outer* `-}` rather than the inner one.
	after := spans[nested+1]
	if groupsOn(after)[groupComment] {
		t.Fatalf("the line after the nested comment is still comment, so the inner closer ended it:\n  %s\n  %s",
			lines[nested+1], render(lines[nested+1], after))
	}
	if !groupsOn(after)[groupKeyword] {
		t.Fatalf("the line after the nested comment has no keyword, so it is not being read as code:\n  %s\n  %s",
			lines[nested+1], render(lines[nested+1], after))
	}

	// The pragma on line one is a keyword and not a comment, even though it opens with
	// the same two characters a comment does.
	if got := groupsOn(spans[0]); got[groupComment] || !got[groupKeyword] {
		t.Fatalf("the LANGUAGE pragma is not being read as a pragma:\n  %s\n  %s",
			lines[0], render(lines[0], spans[0]))
	}
}

// Prolog: the block comment spans two lines, and the character-code notation `0'a`
// must not open a quoted atom -- which would swallow the rest of the line.
func TestHighlightFile_prologBlockCommentAndCharacterCodes(t *testing.T) {
	lines, spans := loadHighlightSample(t, "sample.pl")

	opened := -1
	for index, line := range lines {
		if strings.HasPrefix(line, "/*") {
			opened = index
		}
	}
	if opened < 0 {
		t.Fatal("the fixture no longer holds the block comment this test is about")
	}
	// Both lines of it are comment: the opener's and the closer's.
	for offset := 0; offset < 2; offset++ {
		index := opened + offset
		if got := groupsOn(spans[index]); len(got) != 1 || !got[groupComment] {
			t.Fatalf("line %d of the block comment is not all comment:\n  %s\n  %s",
				offset, lines[index], render(lines[index], spans[index]))
		}
	}
	// And the first non-blank line after it is code.
	next := opened + 2
	for next < len(lines) && strings.TrimSpace(lines[next]) == "" {
		next++
	}
	if groupsOn(spans[next])[groupComment] {
		t.Fatalf("the block comment did not close:\n  %s\n  %s",
			lines[next], render(lines[next], spans[next]))
	}

	// The character-code line: `0'a` and `0'\n` are numbers, and the quoted atom after
	// them is still a string -- which together prove the quote in `0'a` was consumed as
	// part of the number rather than opening a region.
	code := -1
	for index, line := range lines {
		if strings.Contains(line, `0'a`) {
			code = index
		}
	}
	if code < 0 {
		t.Fatal("the fixture no longer holds the 0'a notation this test is about")
	}
	line, lineSpans := lines[code], spans[code]
	for _, token := range []string{`0'a`, `0'\n`} {
		at := strings.Index(line, token)
		if group := groupAt(lineSpans, at); group != groupNumber {
			t.Errorf("%s is group %d, want a number:\n  %s\n  %s",
				token, group, line, render(line, lineSpans))
		}
	}
	if at := strings.Index(line, `'world'`); groupAt(lineSpans, at) != groupString {
		t.Errorf("the quoted atom is not a string, so the character codes broke the scan:\n  %s\n  %s",
			line, render(line, lineSpans))
	}
	// Nothing on that line is a comment, which would be the symptom of a runaway
	// region reaching the end.
	if groupsOn(lineSpans)[groupComment] {
		t.Errorf("something on the character-code line is a comment:\n  %s\n  %s",
			line, render(line, lineSpans))
	}
}

func groupAt(spans []highlightSpan, offset int) highlightGroup {
	for _, span := range spans {
		if offset >= span.start && offset < span.end {
			return span.group
		}
	}
	return groupNone
}
