package applets

import (
	"bufio"
	"io"
	"strings"

	"github.com/rivo/uniseg"
)

// diff -y, GNU's side-by-side format (side.c), which busybox's diff has not got. Each line of
// the left file is printed in the left half and its counterpart in the right, with a gutter
// between: nothing for a line the same on both, `|` for one changed, `<` for one only on the
// left and `>` for one only on the right. A change of n lines on the left and m on the right
// pairs the first min(n, m) with `|`, then gives the rest of the right's `>` and of the left's
// `<`, in that order, as GNU's print_sdiff_hunk does.

// diffSide is -y's layout: GNU's sdiff_half_width and sdiff_column2_offset (diff.c), from -W and
// whether -t expands tabs, and --suppress-common-lines and --left-column.
type diffSide struct {
	half, column2              int
	expandTabs                 bool
	suppressCommon, leftColumn bool
	color                      bool
}

// diffSideTab is GNU's tabsize, the stop a tab in a line moves to and the padding is made of.
const diffSideTab = 8

// newDiffSide lays out a width, as GNU's diff.c does: the right column starts at a tab stop
// near the middle, and each half leaves room for the gutter.
func newDiffSide(width int, expandTabs bool) diffSide {
	tab := diffSideTab
	if expandTabs {
		tab = 1
	}
	const gutter = 3 // GUTTER_WIDTH_MINIMUM
	offset := (width + tab + gutter) / (2 * tab) * tab
	side := diffSide{half: max(0, min(offset-gutter, width-offset)), column2: width, expandTabs: expandTabs}
	if side.half > 0 {
		side.column2 = offset
	}
	return side
}

// writeSideBySide writes every line of both files, as GNU's does even when they are the same.
func (s diffSide) writeSideBySide(under io.Writer, edits []diffEdit, right []string) error {
	out := &diffSideWriter{diffSide: s, out: bufio.NewWriterSize(under, 32<<10)}
	for index := 0; index < len(edits); {
		edit := edits[index]
		if edit.kind == editKeep {
			switch {
			case s.suppressCommon:
			case s.leftColumn:
				out.line(edit.text, true, '(', "", false)
			default:
				out.line(edit.text, true, ' ', right[edit.rightLine-1], true)
			}
			index++
			continue
		}
		var removed, added []string
		for ; index < len(edits) && edits[index].kind != editKeep; index++ {
			if edits[index].kind == editRemove {
				removed = append(removed, edits[index].text)
			} else {
				added = append(added, edits[index].text)
			}
		}
		paired := min(len(removed), len(added))
		for line := range paired {
			out.line(removed[line], true, '|', added[line], true)
		}
		for _, text := range added[paired:] {
			out.line("", false, '>', text, true)
		}
		for _, text := range removed[paired:] {
			out.line(text, true, '<', "", false)
		}
	}
	return out.out.Flush()
}

type diffSideWriter struct {
	diffSide
	out *bufio.Writer
}

// line is GNU's print_1sdiff_line. A gutter of `|` between two lines of which only one ends with
// a newline is `\` when the left has none and `/` when the right has none, and the line ends with
// a newline when either did.
func (w *diffSideWriter) line(left string, hasLeft bool, gutter byte, right string, hasRight bool) {
	colored := w.color && (gutter == '<' || gutter == '>')
	if colored {
		w.out.WriteString(diffColorFor(gutter == '<'))
	}
	newline, column := false, 0
	if hasLeft {
		newline = strings.HasSuffix(left, "\n")
		column = w.half(left, 0)
	}
	if gutter != ' ' {
		column = w.tabTo(column, (w.diffSide.half+w.column2-1)/2) + 1
		if gutter == '|' && newline != strings.HasSuffix(right, "\n") {
			gutter = '\\'
			if newline {
				gutter = '/'
			}
		}
		w.out.WriteByte(gutter)
	}
	if hasRight {
		newline = newline || strings.HasSuffix(right, "\n")
		// An empty line is not padded to the right column, as GNU's is not.
		if !strings.HasPrefix(right, "\n") {
			w.half(right, w.tabTo(column, w.column2))
		}
	}
	if newline {
		w.out.WriteByte('\n')
	}
	if colored {
		w.out.WriteString("\x1b[0m")
	}
}

// half is GNU's print_half_line: a line cut at the half width, counted in columns, where a
// character is as wide as uniseg says, a tab moves to the next stop and is written as a tab
// unless -t, and a carriage return starts the column again, padded to indent.
func (w *diffSideWriter) half(line string, indent int) int {
	bound := w.diffSide.half
	in, out, state := 0, 0, -1
	for text := strings.TrimSuffix(line, "\n"); text != ""; {
		switch c := text[0]; c {
		case '\t':
			// Counted from the half's start, not the screen's: under -t the right half starts
			// off a stop, and GNU's expands `tab\there` there to five spaces, not seven.
			spaces := diffSideTab - in%diffSideTab
			if in == out {
				stop := out + spaces
				if w.expandTabs {
					for stop = min(stop, bound); out < stop; out++ {
						w.out.WriteByte(' ')
					}
				} else if stop < bound {
					out = stop
					w.out.WriteByte('\t')
				}
			}
			in += spaces
		case '\r':
			w.out.WriteByte('\r')
			w.tabTo(0, indent)
			in, out = 0, 0
		case '\b':
			if in == 0 {
				break
			}
			switch in--; {
			case in >= bound:
			case out <= in:
				// Spaces for a tab that was past the bound and so not written.
				for ; out < in; out++ {
					w.out.WriteByte(' ')
				}
			default:
				out = in
				w.out.WriteByte('\b')
			}
		case '\f', '\v':
			if in < bound {
				w.out.WriteByte(c)
			}
		default:
			cluster, rest, boundaries, next := uniseg.StepString(text, state)
			if in += boundaries >> uniseg.ShiftWidth; in <= bound {
				out = in
				w.out.WriteString(cluster)
			}
			text, state = rest, next
			continue
		}
		text, state = text[1:], -1
	}
	return out
}

// tabTo is GNU's tab_from_to: tabs to the last stop at or before to, unless -t, and spaces the
// rest of the way.
func (w *diffSideWriter) tabTo(from, to int) int {
	if !w.expandTabs {
		for stop := from + diffSideTab - from%diffSideTab; stop <= to; stop += diffSideTab {
			w.out.WriteByte('\t')
			from = stop
		}
	}
	for ; from < to; from++ {
		w.out.WriteByte(' ')
	}
	return to
}
