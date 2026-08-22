package applets

import "fmt"

// Applying hunks, which is the half where being strict matters.
//
// A hunk that does not match is refused with its line number. patch's
// traditional fuzz -- shifting a hunk up and down until the context lines happen
// to line up -- is how a patch lands somewhere it was never meant to, and the
// wrong place usually still compiles. Refusing is the answer somebody can act on.

// applyHunks rewrites the lines, hunk by hunk.
//
// Hunks are applied in order and each is matched at the position its header
// claims, adjusted by how much earlier hunks have shifted the file. That offset is
// the only flexibility here: it is arithmetic rather than searching.
func applyHunks(lines []string, hunks []patchHunk, reverse bool) ([]string, error) {
	result := append([]string{}, lines...)
	offset := 0
	for number, hunk := range hunks {
		expected, replacement := hunkSides(hunk, reverse)
		start := hunk.leftStart
		if reverse {
			start = hunk.rightStart
		}
		// A hunk header counts from one, and a zero start means an insertion into
		// an empty file.
		at := max(0, start-1+offset)
		if err := checkHunkContext(result, at, expected, number+1); err != nil {
			return nil, err
		}
		tail := append([]string{}, result[min(at+len(expected), len(result)):]...)
		result = append(result[:at], append(replacement, tail...)...)
		offset += len(replacement) - len(expected)
	}
	return result, nil
}

// hunkSides splits a hunk into what it expects to find and what it puts there.
//
// -R swaps them, which is all reversing a patch is: the removals become the
// additions and the context stays where it is.
func hunkSides(hunk patchHunk, reverse bool) (expected, replacement []string) {
	for _, line := range hunk.lines {
		marker, text := byte(' '), ""
		if line != "" {
			marker, text = line[0], line[1:]
		}
		before, after := marker == ' ' || marker == '-', marker == ' ' || marker == '+'
		if reverse {
			before, after = marker == ' ' || marker == '+', marker == ' ' || marker == '-'
		}
		if before {
			expected = append(expected, text)
		}
		if after {
			replacement = append(replacement, text)
		}
	}
	return expected, replacement
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
