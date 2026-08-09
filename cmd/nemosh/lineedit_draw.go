package main

import (
	"fmt"
	"sort"
	"strings"
)

// redraw repaints the edited line after the prompt.
//
// The whole line is rewritten rather than patched. A patch would have to know
// how wide every character it passed over was, which is the accounting that
// breaks busybox's editor over CJK; rewriting makes the width question arise
// once, in cursorColumns.
//
// Wrapping is why this counts rows. A carriage return returns to the start of
// the *current* row, so once the line is longer than the terminal the prompt's
// row is above the cursor and rewriting from there would paint over the wrong
// rows. The previous draw's row count is remembered and climbed back up.
func (e *lineEditor) redraw(prompt string) {
	promptWidth := promptColumns(lastPromptLine(prompt))
	columns := e.buffer.columns()
	width := e.columnsOrDefault()

	var out strings.Builder
	// Back up to the row the prompt is on, then to its start.
	if e.drawnRows > 0 {
		fmt.Fprintf(&out, "\033[%dA", e.drawnRows)
	}
	out.WriteString("\r")
	if promptWidth > 0 {
		fmt.Fprintf(&out, "\033[%dC", promptWidth)
	}
	out.WriteString(e.buffer.String())
	// Erase to the end of the display rather than padding with spaces: a
	// shrinking line can leave a tail on the rows below, and spaces would have
	// to be counted across the wrap to reach it.
	out.WriteString("\033[J")

	// The cursor is now at the end of what was written. Move it to where it
	// belongs, by row first and then by column.
	endRow := (promptWidth + columns) / width
	targetRow, targetColumn := e.cursorPosition(prompt)
	if up := endRow - targetRow; up > 0 {
		fmt.Fprintf(&out, "\033[%dA", up)
	}
	out.WriteString("\r")
	if targetColumn > 0 {
		fmt.Fprintf(&out, "\033[%dC", targetColumn)
	}

	fmt.Fprint(e.screen, out.String())
	e.drawn = columns
	e.drawnRows = targetRow
}

// cursorPosition is where the terminal cursor belongs: how many rows below the
// prompt's row, and how many columns across.
func (e *lineEditor) cursorPosition(prompt string) (int, int) {
	width := e.columnsOrDefault()
	total := promptColumns(lastPromptLine(prompt)) + e.buffer.cursorColumns()
	return total / width, total % width
}

// lastPromptLine is the part of the prompt the edited line shares a row with. A
// multi-line prompt -- the default ends with a newline before the symbol -- only
// contributes its final row to the column the text starts in.
func lastPromptLine(prompt string) string {
	if index := strings.LastIndexByte(prompt, '\n'); index >= 0 {
		return prompt[index+1:]
	}
	return prompt
}

// complete handles Tab. A single candidate is inserted whole and followed by a
// blank, so the next word can be typed straight away. Several candidates insert
// only what they share, and are listed: taking the user as far as the choice
// actually is, without choosing for them.
func (e *lineEditor) complete(prompt string) {
	// The word as typed and the word as a filename are not the same string once
	// a blank has been escaped: on screen `My\ Do`, on disk `My Do`. Matching
	// uses the second, and replacing uses the first, because that is what is
	// actually on screen to be deleted.
	typed := e.buffer.currentWord()
	stem := unescapeTypedWord(typed)
	prefix := e.buffer.currentWordPrefix()

	var matches []string
	if completesCommand(prefix) {
		matches = completeCommand(stem)
	} else {
		matches = completeOperand(e.workingDirectory, commandInProgress(prefix), stem)
	}
	if len(matches) == 0 {
		return
	}
	if len(matches) == 1 {
		e.replaceWord(typed, escapeForInsertion(matches[0]))
		if !strings.HasSuffix(matches[0], "/") {
			e.buffer.insert(' ')
		}
		return
	}
	if shared := longestSharedPrefix(matches); len(shared) > len(stem) {
		e.replaceWord(typed, escapeForInsertion(shared))
	}
	// Listed unescaped: the backslashes are how the shell reads the name, not
	// how the name is spelled, and a column of them is harder to read.
	e.listCandidates(matches, prompt)
}

// replaceWord swaps the word under the cursor for the completion.
func (e *lineEditor) replaceWord(word, replacement string) {
	for range []rune(word) {
		e.buffer.backspace()
	}
	for _, r := range replacement {
		e.buffer.insert(r)
	}
}

// listCandidates prints the choices above a fresh prompt, which is what a user
// expects a second Tab to show.
func (e *lineEditor) listCandidates(matches []string, prompt string) {
	sort.Strings(matches)
	fmt.Fprintln(e.screen)
	fmt.Fprintln(e.screen, strings.Join(matches, "  "))
	fmt.Fprint(e.screen, prompt)
	e.resetDrawState()
}
