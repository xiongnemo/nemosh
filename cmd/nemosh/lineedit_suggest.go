package main

// suggestionFor is what will be drawn grey after the cursor, already cut to fit.
//
// Two rules make this safe to add to a redraw that was already correct.
//
// It is only offered when the cursor is at the end of the line. A suggestion is
// a guess about what comes next, and there is no "next" when the cursor is in
// the middle of the text; drawing one past the end while editing the middle
// would put the two in different places on screen and mean nothing.
//
// It never wraps. Truncating to the columns left on the current row means the
// line's last row is still the line's last row, so every number the redraw
// computes from the buffer -- where the cursor goes, how many rows to climb next
// time -- is exactly what it was before suggestions existed. fish lets its
// suggestions wrap; doing that here would put the wrap arithmetic, which this
// editor has already had two defects in, on the keystroke path for a decoration.
func (e *lineEditor) suggestionFor(promptWidth, bufferColumns, width int) string {
	if !e.styling.suggests() {
		return ""
	}
	if e.buffer.cursor != e.buffer.length() {
		return ""
	}
	text := suggester{
		history:  e.history,
		commands: e.commands.candidates(),
		hosts:    e.hosts.candidates(),
		specs:    e.specs,
	}.suggest(e.buffer.String())
	if text == "" {
		return ""
	}
	// One column is kept back. Filling the row exactly leaves the cursor in the
	// last cell or on the next row depending on the terminal, and which of those
	// happens is not worth depending on.
	remaining := width - (promptWidth+bufferColumns)%width - 1
	if remaining <= 0 {
		return ""
	}
	return truncateToColumns(text, remaining)
}

// truncateToColumns cuts text to fit, counting cells rather than characters and
// never splitting a rune.
func truncateToColumns(text string, limit int) string {
	used := 0
	for index, r := range text {
		columns := runeColumns(r)
		if used+columns > limit {
			return text[:index]
		}
		used += columns
	}
	return text
}

// acceptSuggestion takes what was offered, and reports whether there was
// anything to take.
//
// Only an explicit key does this, never Enter. The suggestion is not in the
// buffer, so Enter submits what was typed and could not do otherwise -- which is
// the property that makes drawing a guess safe at all.
func (e *lineEditor) acceptSuggestion() bool {
	if e.suggestion == "" || e.buffer.cursor != e.buffer.length() {
		return false
	}
	for _, r := range e.suggestion {
		e.buffer.insert(r)
	}
	e.suggestion = ""
	return true
}
