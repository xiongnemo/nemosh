package applets

import "github.com/gdamore/tcell/v2"

// Lines shown at the foot of the screen until Enter, busybox's Hit_Return: what :p prints over
// several lines, :set all, and what a :! command leaves. One line is shown on the status line
// alone, as busybox and vim show one.

// showLines shows lines, on the status line if there is one, or above it until Enter.
func (e *viEditor) showLines(lines []string) {
	switch len(lines) {
	case 0:
	case 1:
		e.say(false, "%s", lines[0])
	default:
		e.shown = lines
	}
}

// dismissKey is a key typed while lines are shown, and says whether it was taken: Enter, as
// busybox's Hit_Return waits for, or Space or Escape, which put them away too.
func (e *viEditor) dismissKey(key rune) bool {
	if len(e.shown) == 0 {
		return false
	}
	if key == viEnter || key == '\n' || key == ' ' || key == viEscape {
		e.shown = nil
	}
	return true
}

// drawShown draws the lines being shown over the foot of the screen, the prompt below them.
func (e *viEditor) drawShown() {
	rows := min(len(e.shown), e.height-1)
	top := e.height - 1 - rows
	for index, line := range e.shown[len(e.shown)-rows:] {
		e.drawPlain(top+index, line, tcell.StyleDefault)
	}
	e.drawPlain(e.height-1, "[Hit return to continue]", tcell.StyleDefault.Reverse(true))
	e.screen.ShowCursor(min(len("[Hit return to continue]"), e.width-1), e.height-1)
}

// drawPlain writes text on row y from its start, clearing the rest of the row.
func (e *viEditor) drawPlain(y int, text string, style tcell.Style) {
	for x := range e.width {
		e.screen.SetContent(x, y, ' ', nil, tcell.StyleDefault)
	}
	x := 0
	for _, r := range text {
		if x >= e.width {
			return
		}
		e.screen.SetContent(x, y, r, nil, style)
		x++
	}
}
