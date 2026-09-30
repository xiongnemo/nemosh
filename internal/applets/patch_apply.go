package applets

import (
	"fmt"
	"strings"
)

// Applying hunks, which is the half where being strict matters.
//
// A hunk that does not match is refused with its line number. patch's
// traditional fuzz -- shifting a hunk up and down until the context lines happen
// to line up -- is how a patch lands somewhere it was never meant to, and the
// wrong place usually still compiles. Refusing is the answer somebody can act on.
// busybox's patch searches forward for a hunk's context and applies it where it
// is found; this does not.

// patchText is a file as its lines, and whether the last of them ends in a newline.
type patchText struct {
	lines   []string
	newline bool
}

func readPatchText(content string) patchText {
	return patchText{lines: splitPatchLines(content), newline: content == "" || strings.HasSuffix(content, "\n")}
}

// String is the file again: an emptied one is empty, not a newline, as it was written before.
func (t patchText) String() string {
	if len(t.lines) == 0 {
		return ""
	}
	if t.newline {
		return strings.Join(t.lines, "\n") + "\n"
	}
	return strings.Join(t.lines, "\n")
}

func splitPatchLines(text string) []string {
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// applyHunks rewrites the text, hunk by hunk.
//
// Hunks are applied in order and each is matched at the position its header
// claims, adjusted by how much earlier hunks have shifted the file. That offset is
// the only flexibility here: it is arithmetic rather than searching. A hunk that
// ends at the end of the file says whether the last line has a newline, which is
// what `\ No newline at end of file` is for.
func applyHunks(text patchText, hunks []patchHunk, reverse bool) (patchText, error) {
	result := append([]string{}, text.lines...)
	newline := text.newline
	offset := 0
	for number, hunk := range hunks {
		if hunk.short {
			return patchText{}, fmt.Errorf("hunk #%d ends before its header says it does", number+1)
		}
		side := hunkSides(hunk, reverse)
		// A hunk header counts from one, and a hunk that expects nothing goes after its start
		// line: `@@ -0,0 +1 @@` begins an empty file, and `@@ -3,0 +4 @@` adds after line 3.
		at := side.start - 1 + offset
		if len(side.expected) == 0 {
			at = side.start + offset
		}
		at = min(max(0, at), len(result))
		if err := checkHunkContext(result, at, side.expected, number+1); err != nil {
			return patchText{}, err
		}
		end := at + len(side.expected)
		if end == len(result) {
			newline = len(side.replacement) == 0 || !side.noNewline
		}
		tail := append([]string{}, result[end:]...)
		result = append(result[:at], append(side.replacement, tail...)...)
		offset += len(side.replacement) - len(side.expected)
	}
	return patchText{lines: result, newline: newline}, nil
}

// hunkSide is one direction of a hunk: where it starts, what it expects to find there and
// what it puts there, and whether what it puts ends without a newline.
type hunkSide struct {
	start                 int
	expected, replacement []string
	noNewline             bool
}

// hunkSides splits a hunk into what it expects to find and what it puts there.
//
// -R swaps them, which is all reversing a patch is: the removals become the
// additions and the context stays where it is.
func hunkSides(hunk patchHunk, reverse bool) hunkSide {
	side := hunkSide{start: hunk.oldStart, noNewline: hunk.newNoNewline}
	if reverse {
		side = hunkSide{start: hunk.newStart, noNewline: hunk.oldNoNewline}
	}
	for _, line := range hunk.lines {
		marker, text := line[0], line[1:]
		before, after := marker == ' ' || marker == '-', marker == ' ' || marker == '+'
		if reverse {
			before, after = after, before
		}
		if before {
			side.expected = append(side.expected, text)
		}
		if after {
			side.replacement = append(side.replacement, text)
		}
	}
	return side
}

// checkHunkContext refuses unless the file really holds what the hunk expects.
//
// The comparison is exact. Reporting the line number and the first line that did
// not match is what makes a rejection diagnosable -- "hunk failed" on its own
// sends somebody reading the whole file.
func checkHunkContext(lines []string, at int, expected []string, number int) error {
	if at+len(expected) > len(lines) {
		return fmt.Errorf("hunk #%d failed at line %d: the file ends before the hunk does", number, at+1)
	}
	for index, want := range expected {
		if lines[at+index] != want {
			return fmt.Errorf("hunk #%d failed at line %d: expected %q but found %q",
				number, at+index+1, want, lines[at+index])
		}
	}
	return nil
}
