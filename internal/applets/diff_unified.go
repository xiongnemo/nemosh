package applets

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// The unified output format: the header, and the hunks.
//
//	--- old
//	+++ new
//	@@ -1,5 +1,5 @@
//	 context
//	-removed
//	+added
//
// A hunk covers a run of changes plus `context` lines either side, and two runs
// close enough to share context are merged into one hunk -- which is why the
// grouping is computed before anything is written rather than emitted as the edit
// script is walked.

// The header names the files, but where -L labels them: the first -L the first file, and the
// last of the others the second, as busybox takes them. -T puts a tab after each line's marker,
// and -t writes a tab as the spaces to the next multiple of eight, counted from the line's start.
// All three were taken and ignored.
//
// It is written through a buffer, flushed once the file's diff is written, as busybox's stdio
// writes it. A write a line, to a file, made the 6.8 MB diff of two of Go's ssa files take 24 s
// where busybox takes 0.5; nothing else is said while it is written, so nothing is out of order.
func (r diffRequest) writeUnified(under io.Writer, edits []diffEdit, hunks []diffHunk) error {
	stdout := bufio.NewWriterSize(under, 32<<10)
	if err := r.writeUnifiedTo(stdout, edits, hunks); err != nil {
		return err
	}
	return stdout.Flush()
}

func (r diffRequest) writeUnifiedTo(stdout io.Writer, edits []diffEdit, hunks []diffHunk) error {
	left, right := r.left, r.right
	if r.absent[0] {
		left = "/dev/null"
	}
	if r.absent[1] {
		right = "/dev/null"
	}
	if len(r.labels) > 0 {
		left = r.labels[0]
	}
	if len(r.labels) > 1 {
		right = r.labels[len(r.labels)-1]
	}
	// No timestamps in the header: busybox omits them, and a timestamp would make
	// the output differ between two runs over unchanged files.
	if _, err := fmt.Fprintf(stdout, "%s\n%s\n", r.colored(diffColorHeader, "--- "+left), r.colored(diffColorHeader, "+++ "+right)); err != nil {
		return err
	}
	for _, hunk := range hunks {
		if err := r.writeDiffHunk(stdout, edits[hunk.from:hunk.to]); err != nil {
			return err
		}
	}
	return nil
}

// diffLineText is a line as a hunk writes it, after its marker.
func (r diffRequest) diffLineText(text string) string {
	if r.expandTabs && strings.IndexByte(text, '\t') >= 0 {
		var expanded strings.Builder
		for index := 0; index < len(text); index++ {
			if text[index] != '\t' {
				expanded.WriteByte(text[index])
				continue
			}
			for expanded.WriteByte(' '); expanded.Len()%8 != 0; {
				expanded.WriteByte(' ')
			}
		}
		text = expanded.String()
	}
	if r.initialTab {
		return "\t" + text
	}
	return text
}

// colored is text in color and reset, under --color; a line of context, which has no color, and
// everything without --color are text alone.
func (r diffRequest) colored(color, text string) string {
	if !r.side.color || color == "" {
		return text
	}
	return color + text + diffColorReset
}

// diffHunk is a half-open range of the edit script.
type diffHunk struct{ from, to int }

// groupDiffHunks finds the ranges worth printing.
//
// A changed line pulls in `context` lines either side; ranges that touch or
// overlap merge, so a file with two nearby edits produces one hunk rather than
// two that repeat the same context lines.
func groupDiffHunks(edits []diffEdit, context int) []diffHunk {
	var hunks []diffHunk
	for index := 0; index < len(edits); index++ {
		if edits[index].kind == editKeep {
			continue
		}
		from := max(0, index-context)
		// Walk forward over everything still within reach, extending as further
		// changes are found.
		to := index + 1
		for scan := index; scan < len(edits); scan++ {
			if edits[scan].kind != editKeep {
				to = scan + 1
				continue
			}
			if scan-to >= context {
				break
			}
		}
		to = min(len(edits), to+context)
		if len(hunks) > 0 && from <= hunks[len(hunks)-1].to {
			hunks[len(hunks)-1].to = max(hunks[len(hunks)-1].to, to)
		} else {
			hunks = append(hunks, diffHunk{from: from, to: to})
		}
		index = to - 1
	}
	return hunks
}

func (r diffRequest) writeDiffHunk(stdout io.Writer, edits []diffEdit) error {
	leftStart, leftCount, rightStart, rightCount := hunkRanges(edits)
	numbers := fmt.Sprintf("@@ -%s +%s @@", hunkRange(leftStart, leftCount), hunkRange(rightStart, rightCount))
	if _, err := fmt.Fprintf(stdout, "%s\n", r.colored(diffColorNumber, numbers)); err != nil {
		return err
	}
	for _, edit := range edits {
		marker, color := " ", ""
		switch edit.kind {
		case editRemove:
			marker, color = "-", diffColorFor(true)
		case editAdd:
			marker, color = "+", diffColorFor(false)
		}
		// A line keeps its newline, so a last line without one is said to have none, as
		// busybox and every diff say it, and patch reads it.
		text, ended := strings.CutSuffix(edit.text, "\n")
		if _, err := fmt.Fprintf(stdout, "%s\n", r.colored(color, marker+r.diffLineText(text))); err != nil {
			return err
		}
		if !ended {
			if _, err := fmt.Fprintln(stdout, `\ No newline at end of file`); err != nil {
				return err
			}
		}
	}
	return nil
}

// hunkRanges is where each side of a hunk starts and how many of its lines the hunk holds. A
// side the hunk holds none of, which under -U0 a hunk of additions or of removals alone is,
// starts at the line it follows, as busybox prints it: `@@ -2,0 +3,5 @@`. It was 0, `-0,0`,
// which says the lines were added before the left's first.
func hunkRanges(edits []diffEdit) (leftStart, leftCount, rightStart, rightCount int) {
	leftStart, rightStart = edits[0].leftLine, edits[0].rightLine
	for _, edit := range edits {
		if edit.kind != editAdd {
			if leftCount == 0 {
				leftStart = edit.leftLine
			}
			leftCount++
		}
		if edit.kind != editRemove {
			if rightCount == 0 {
				rightStart = edit.rightLine
			}
			rightCount++
		}
	}
	return leftStart, leftCount, rightStart, rightCount
}

// hunkRange formats one side of the `@@` line.
//
// A count of one is written without it -- `@@ -3 +3,2 @@` -- which is the format's
// rule and what patch expects. A count of zero carries the line *before* the
// insertion point, which is why an empty side is `0,0` only when the file itself
// is empty.
func hunkRange(start, count int) string {
	if count == 1 {
		return fmt.Sprintf("%d", start)
	}
	if count == 0 && start == 0 {
		return "0,0"
	}
	return fmt.Sprintf("%d,%d", start, count)
}
