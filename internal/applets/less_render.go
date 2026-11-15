package applets

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

// Drawing one line as busybox's less draws it.
//
// Each rune was put in a cell of its own and the column moved on by one, which was right for
// ASCII and nothing else: a tab was a cell holding a tab, ESC from a coloured `ls` was a cell
// holding ESC for the terminal to make what it would of, and a Chinese character, two columns
// wide, had the next character drawn over its second half. Now:
//
//   - a control character is the letter for it, reversed, as busybox prints it
//     (miscutils/less.c, print_ascii): ESC is `[`, BEL is `G`, DEL is `?`;
//   - a tab goes to the next stop of eight, as busybox counts it in read_lines;
//   - a character is as wide as its grapheme cluster, by uniseg, the rule the editor and tview
//     already use, so the columns after it are the columns it leaves.
//
// A match is reversed by the rune it starts and ends at, which matchSpans counts, so a match
// after a tab or a wide character is reversed where it is drawn.

// drawLine draws text from column 0 of row, prefix plain before it, reversing what spans
// cover. It stops at the screen's edge.
func (p *lessPager) drawLine(row int, prefix, text string, spans [][2]int) {
	plain := tcell.StyleDefault
	column := p.putPlain(row, 0, prefix, plain)
	runeIndex := len([]rune(prefix))
	state := -1
	for rest := text; rest != "" && column < p.width; {
		cluster, remaining, boundaries, newState := uniseg.StepString(rest, state)
		rest, state = remaining, newState
		style := styleForColumn(plain, plain.Reverse(true), runeIndex, spans)
		runeIndex += len([]rune(cluster))
		first := []rune(cluster)[0]
		switch {
		case first == '\t':
			for stop := (column/8 + 1) * 8; column < stop && column < p.width; column++ {
				p.screen.Put(column, row, " ", style)
			}
		case first < 0x20 || first == 0x7f || first == 0x9b:
			// Reversed against whatever the cell would have been: a control character inside a
			// match comes out plain, which is how print_hilite inside a highlight reads too.
			p.screen.Put(column, row, lessControlLetter(first), style.Reverse(style == plain))
			column++
		default:
			width := boundaries >> uniseg.ShiftWidth
			if width < 1 {
				width = 1
			}
			if column+width > p.width {
				return
			}
			p.screen.Put(column, row, cluster, style)
			column += width
		}
	}
}

// putPlain draws text that has no controls in it, a column a rune, and answers the column after.
func (p *lessPager) putPlain(row, column int, text string, style tcell.Style) int {
	for _, character := range text {
		if column >= p.width {
			break
		}
		p.screen.Put(column, row, string(character), style)
		column++
	}
	return column
}

// lessControlLetter is the letter busybox prints for a control character: the one it is the
// control of, `@` for NUL, `?` for DEL, and `{` for the CSI byte, which busybox's own comment
// calls an infamous sin.
func lessControlLetter(char rune) string {
	switch char {
	case 0x7f:
		return "?"
	case 0x9b:
		return "{"
	}
	return string(rune('@' + char))
}
