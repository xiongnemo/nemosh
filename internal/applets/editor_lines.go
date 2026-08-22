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

// lines is the buffer split by line, and which line the cursor is on.
func (v *editorView) lines() ([]string, int) {
	text := v.area.GetText()
	lines := strings.Split(text, "\n")
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
	lines := strings.Split(v.area.GetText(), "\n")
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
