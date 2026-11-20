package applets

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

// Drawing, as busybox's format_line and refresh draw: a row a line, never wrapped, `~` past
// the end, a tab as the spaces to the next tabstop, a control character as ^X and DEL as ^?.
// A line wider than the screen is scrolled sideways to keep the cursor on it.

// viGlyph is how r is drawn starting at display column column: what is shown, and the cells it
// takes. A rune of no width combines with the cell before it.
func viGlyph(r rune, column, tabstop int) ([]rune, int) {
	switch {
	case r == '\t':
		width := tabstop - column%tabstop
		spaces := make([]rune, width)
		for index := range spaces {
			spaces[index] = ' '
		}
		return spaces, width
	case r < ' ' || r == 0x7f:
		return []rune{'^', r ^ 0x40}, 2
	}
	return []rune{r}, uniseg.StringWidth(string(r))
}

// displayColumn is where the rune at col begins on the screen, and where the cursor is drawn on
// it: on the last cell of a tab in command mode, as busybox and vim draw it, and on its first
// in insert mode, where typing goes before it.
func (e *viEditor) displayColumn(line []rune, col int) int {
	column := 0
	for index := 0; index < col && index < len(line); index++ {
		_, width := viGlyph(line[index], column, e.tabstop)
		column += width
	}
	if col < len(line) && line[col] == '\t' && e.mode == viNormal {
		_, width := viGlyph('\t', column, e.tabstop)
		column += width - 1
	}
	return column
}

// textRows is the rows the buffer is drawn in: all but the status line.
func (e *viEditor) textRows() int { return max(1, e.height-1) }

func (e *viEditor) draw() {
	e.scrollToCursor()
	e.screen.Clear()
	for y := range e.textRows() {
		index := e.top + y
		if index >= len(e.lines) {
			e.screen.SetContent(0, y, '~', nil, tcell.StyleDefault)
			continue
		}
		e.drawLine(y, e.lines[index])
	}
	e.drawStatus()
	e.screen.Show()
}

func (e *viEditor) drawLine(y int, line []rune) {
	column, previous := 0, -1
	for _, r := range line {
		shown, width := viGlyph(r, column, e.tabstop)
		if width == 0 {
			if previous >= 0 {
				main, combining, style, _ := e.screen.GetContent(previous, y)
				e.screen.SetContent(previous, y, main, append(combining, r), style)
			}
			continue
		}
		for cell, glyph := range shown {
			x := column - e.left + cell
			if width == 2 && len(shown) == 1 && cell == 0 && x+1 >= e.width {
				break
			}
			if x >= 0 && x < e.width {
				e.screen.SetContent(x, y, glyph, nil, tcell.StyleDefault)
				previous = x
			}
		}
		column += width
		if column-e.left >= e.width {
			return
		}
	}
}

// drawStatus is the bottom line: the line being typed after : or /, a message once, or else
// busybox's format_edit_status, `- name [Readonly] [Modified] 2/10 20%`, and the cursor.
func (e *viEditor) drawStatus() {
	y := e.textRows()
	style := tcell.StyleDefault
	text := e.status()
	switch {
	case e.mode == viCommandLine:
		text = string(e.command)
	case e.message != "":
		text = e.message
		if e.bold {
			style = style.Reverse(true)
		}
		e.message, e.bold = "", false
	}
	x := 0
	for _, r := range text {
		if x >= e.width {
			break
		}
		e.screen.SetContent(x, y, r, nil, style)
		x += max(1, uniseg.StringWidth(string(r)))
	}
	if e.mode == viCommandLine {
		e.screen.ShowCursor(min(x, e.width-1), y)
		return
	}
	e.screen.ShowCursor(e.displayColumn(e.line(), e.col)-e.left, e.row-e.top)
}

// status is busybox's format_edit_status.
func (e *viEditor) status() string {
	mode := '-'
	switch e.mode {
	case viInsert:
		mode = 'I'
	case viReplace:
		mode = 'R'
	}
	name := e.file.name
	if name == "" {
		name = "No file"
	}
	flags := ""
	if e.file.readOnly {
		flags += " [Readonly]"
	}
	if e.isModified() {
		flags += " [Modified]"
	}
	current, total := e.row+1, len(e.lines)
	return fmt.Sprintf("%c %s%s %d/%d %d%%", mode, name, flags, current, total, 100*current/total)
}
