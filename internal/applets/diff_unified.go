package applets

import (
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
func (r diffRequest) writeUnified(stdout io.Writer, edits []diffEdit, hunks []diffHunk) error {
	left, right := r.left, r.right
	if len(r.labels) > 0 {
		left = r.labels[0]
	}
	if len(r.labels) > 1 {
		right = r.labels[len(r.labels)-1]
	}
	// No timestamps in the header: busybox omits them, and a timestamp would make
	// the output differ between two runs over unchanged files.
	if _, err := fmt.Fprintf(stdout, "--- %s\n+++ %s\n", left, right); err != nil {
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
	if _, err := fmt.Fprintf(stdout, "@@ -%s +%s @@\n",
		hunkRange(leftStart, leftCount), hunkRange(rightStart, rightCount)); err != nil {
		return err
	}
	for _, edit := range edits {
		marker := " "
		switch edit.kind {
		case editRemove:
			marker = "-"
		case editAdd:
			marker = "+"
		}
		if _, err := fmt.Fprintf(stdout, "%s%s\n", marker, r.diffLineText(edit.text)); err != nil {
			return err
		}
	}
	return nil
}

func hunkRanges(edits []diffEdit) (leftStart, leftCount, rightStart, rightCount int) {
	for _, edit := range edits {
		if edit.kind != editAdd {
			if leftStart == 0 {
				leftStart = edit.leftLine
			}
			leftCount++
		}
		if edit.kind != editRemove {
			if rightStart == 0 {
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
