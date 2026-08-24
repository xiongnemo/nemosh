package applets

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// The line-number gutter.
//
// Drawn by the widget that draws the text rather than as a second widget beside it,
// because the number on a row must be the line on that row and nothing else can
// guarantee that. A TextView in a Flex would need the text area's scroll offset copied
// into it every frame, and -- being to its left -- would read that offset *before* the
// area had clamped it, so a fast scroll would show numbers a row out. Here the offset is
// read after `TextArea.Draw` has run, which is the one moment it is known to be settled.
//
// The widget therefore owns a rect wider than the one it hands to tview: SetRect keeps
// the outer one and gives the text area the remainder. That is also what keeps the
// highlighting correct for free -- the colour pass reads `GetInnerRect`, which is the
// text area's, so the columns it paints move with the text rather than needing to know
// the gutter exists.

const (
	// gutterGap is the blank column between the numbers and the text.
	gutterGap = 1
	// gutterMinimumText is how much room the text keeps. Below it the gutter is dropped
	// rather than squeezing the text into nothing: on an eighty-column terminal a
	// six-digit file costs a fourteenth of the width, but on a twenty-five-column one it
	// would cost a quarter, and at that point the numbers are worth less than the text.
	gutterMinimumText = 20
)

// showLineNumbers turns the gutter on. Off at the widget level so that the highlighting
// tests, which assert colours at known columns, are unaffected by it.
func (a *highlightedArea) showLineNumbers(show bool) { a.numbers = show }

// SetRect keeps the full rect and gives the text area what is left of it.
//
// tview's Flex calls this every frame with the outer rect, so the subtraction cannot
// accumulate -- the width handed in is always the full one.
func (a *highlightedArea) SetRect(x, y, width, height int) {
	a.outer = [4]int{x, y, width, height}
	gutter := a.gutterWidth()
	a.TextArea.SetRect(x+gutter, y, width-gutter, height)
}

// gutterWidth is the room the numbers need, or zero when they are off or will not fit.
//
// Sized to the line count rather than to a fixed width, so a short file does not pay
// for a long one: three columns at 99 lines, four at 999. It therefore changes as the
// buffer grows, which is why it is computed rather than stored.
func (a *highlightedArea) gutterWidth() int {
	if !a.numbers {
		return 0
	}
	width := len(strconv.Itoa(max(len(a.lines), 1))) + gutterGap
	if a.outer[2]-width < gutterMinimumText {
		return 0
	}
	return width
}

// drawGutter writes the numbers. Called after TextArea.Draw, so the scroll offset it
// reads is the settled one.
func (a *highlightedArea) drawGutter(screen tcell.Screen) {
	gutter := a.gutterWidth()
	if gutter == 0 {
		return
	}
	x, y, _, height := a.outer[0], a.outer[1], a.outer[2], a.outer[3]
	// Grey, and the same grey a comment gets: the numbers are not the file, and a
	// gutter that competes with the code for attention is worse than none.
	style := a.GetTextStyle().Foreground(tcell.ColorGray)
	rowOffset, _ := a.GetOffset()
	blank := strings.Repeat(" ", gutter)
	for row := 0; row < height; row++ {
		label := blank
		if index := rowOffset + row; index < len(a.lines) {
			// Right-aligned in the number field, then the gap: a column of numbers whose
			// units digits line up can be read at a glance and one whose do not cannot.
			label = fmt.Sprintf("%*d%s", gutter-gutterGap, index+1, strings.Repeat(" ", gutterGap))
		}
		for column, r := range []rune(label) {
			screen.Put(x+column, y+row, string(r), style)
		}
	}
}
