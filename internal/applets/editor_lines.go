package applets

import "strings"

// Line-level operations on the text area.
//
// tview's TextArea holds one string and reports the cursor as a row and column,
// so the line-oriented commands -- cut, paste, search, go-to -- work by splitting
// and rejoining. That is cheap for the sizes an editor like this is used at, and
// it keeps the widget as the single owner of the text rather than maintaining a
// second copy that could drift from it.

// promptLabel and promptText live here to keep editor_view.go under the size
// ceiling; they belong to the view.
type editorPromptState struct {
	promptLabel string
	promptText  string
}

// editorLines splits the buffer into the lines a *reader* would count.
//
// strings.Split("a\nb\n", "\n") answers three elements, the last empty, because a
// newline is a terminator and Split treats it as a separator. So every file ending
// in a newline -- which is every well-formed text file -- had one line more than it
// has, and `^_ 9999` on a sixty-line file answered "Line 61". A test asserting the
// clamp is what found it.
//
// The trailing empty element is dropped, and only when there is something else to
// keep: an empty buffer is one empty line rather than zero lines, and zero would
// divide by len(lines) in the search wrap.
func editorLines(text string) []string {
	lines := strings.Split(text, "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		return lines[:len(lines)-1]
	}
	return lines
}

// lines is the buffer split by line, and which line the cursor is on.
func (v *editorView) lines() ([]string, int) {
	lines := editorLines(v.area.GetText())
	row, _, _, _ := v.area.GetCursor()
	if row < 0 {
		row = 0
	}
	if row >= len(lines) {
		row = len(lines) - 1
	}
	return lines, row
}

// setText replaces the whole buffer and puts the cursor at the start of a line.
//
// Replace over the full range rather than SetText, because SetText discards the
// undo history and losing it to a cut would be worse than the cut being
// slightly slower.
func (v *editorView) setText(text string, row int) {
	v.area.Replace(0, v.area.GetTextLength(), text)
	v.moveToLine(row)
	v.modified = true
	v.refreshTitle()
}

// moveToLine puts the cursor at the beginning of a line, counted from zero.
func (v *editorView) moveToLine(row int) {
	lines := editorLines(v.area.GetText())
	if row < 0 {
		row = 0
	}
	if row >= len(lines) {
		row = max(0, len(lines)-1)
	}
	offset := 0
	for index := range row {
		// **Bytes**, not runes: tview's Select and Replace take index positions
		// into the whole text string. Measured -- GetTextLength answers 10 for
		// two CJK characters plus "ab" and two newlines, which is the byte count
		// and not the six runes. Counting runes here put the cursor in the wrong
		// place on any line holding a multibyte character. The +1 is the newline
		// that Split removed.
		offset += len(lines[index]) + 1
	}
	v.area.Select(offset, offset)
}
