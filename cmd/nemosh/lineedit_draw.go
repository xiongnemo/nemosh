package main

import (
	"fmt"
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

	// The suggestion is computed here, not stored by the keypress that caused
	// it, because it depends on the width the line is being drawn at.
	e.suggestion = e.suggestionFor(promptWidth, columns, width)

	var out strings.Builder
	// Back up to the row the prompt is on, then to its start.
	if e.drawnRows > 0 {
		fmt.Fprintf(&out, "\033[%dA", e.drawnRows)
	}
	out.WriteString("\r")
	if promptWidth > 0 {
		fmt.Fprintf(&out, "\033[%dC", promptWidth)
	}
	// Erase to the end of the display rather than padding with spaces: a
	// shrinking line can leave a tail on the rows below, and spaces would have
	// to be counted across the wrap to reach it. Before the line rather than
	// after it, because a tab in the line moves the cursor over what an earlier
	// drawing left there without erasing it.
	out.WriteString("\033[J")
	// A newline in the line -- a paste, a recalled multi-line command -- starts the next row
	// at the left edge.
	painted := e.styling.paint(e.buffer.String(), e.buffer.cursor, e.suggestion, e.commandStanding)
	out.WriteString(strings.ReplaceAll(painted, "\n", "\r\n"))

	// The cursor is now at the end of what was written. Move it to where it
	// belongs, by row first and then by column.
	endRow, _ := layoutPosition(promptWidth, e.buffer.runes, width)
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
	return layoutPosition(promptColumns(lastPromptLine(prompt)), e.buffer.runes[:e.buffer.cursor], e.columnsOrDefault())
}

// layoutPosition is the row and column after text, drawn from column start of the first row
// on a screen width cells wide: the text wraps at the edge, a newline in it starts the next
// row at the left edge, and a tab goes on to the next tab stop, every eighth column.
func layoutPosition(start int, text []rune, width int) (int, int) {
	row, total := 0, start
	for _, r := range text {
		switch r {
		case '\n':
			row += total/width + 1
			total = 0
		case '\t':
			total += tabStop - total%width%tabStop
		default:
			total += runeColumns(r)
		}
	}
	return row + total/width, total % width
}

// tabStop is how far apart a terminal's tab stops are, unless something has set them.
const tabStop = 8

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
	// A `complete` specification for the command answers first, as bash's does; see
	// complete_programmable.go.
	if e.programmable != nil {
		answer, found := e.programmable(e.buffer.String(), len(string(e.buffer.runes[:e.buffer.cursor])))
		if found && e.completeProgrammatically(answer, prompt) {
			return
		}
	}
	// A `$` asks for a variable's name, which is no path; see complete_variable.go.
	if e.completeVariable(prompt) {
		return
	}
	// The word as typed and the word as a filename are not the same string once
	// a blank has been escaped: on screen `My\ Do`, on disk `My Do`. Matching
	// uses the second, and replacing uses the first, because that is what is
	// actually on screen to be deleted.
	typed := e.buffer.currentWord()
	// And a word with a quote still open is read inside it; see complete_quote.go.
	stem, quote := typedWordValue(typed)
	prefix := e.buffer.currentWordPrefix()

	var matches []string
	var areOptions bool
	operand := !e.completesCommandName(prefix)
	switch {
	case operand:
		matches, areOptions = e.completeOperandWord(prefix, stem)
	case namesAProgramByPath(stem):
		matches = completeProgramPath(e.paths(), stem)
	default:
		matches = completeCommand(stem, e.commands.candidates())
	}
	// Only a path is rewritten, and only on the way in. An option begins with a
	// dash because that is what an option is, so prefixing it would turn
	// `--color` into `./--color` -- the rule for making a filename usable,
	// applied to the one word it was never about. The list below shows names as
	// they are on disk, because that is what the reader is choosing between.
	insert := func(text string) string {
		if operand && !areOptions {
			text = disambiguateOperand(text)
		}
		if quote != 0 {
			return quoteForInsertion(text, quote)
		}
		return escapeForInsertion(text)
	}
	if len(matches) == 0 {
		// Nothing to offer has to be distinguishable from nothing happening.
		// `cd ` in a directory holding no subdirectories is a correct empty
		// answer, and in silence it reads exactly like a broken Tab -- which is
		// how the defect that made every argument uncompletable went unnoticed.
		// busybox rings the bell here (libbb/lineedit.c:1468).
		//
		// Only here, though. busybox also rings it when several candidates
		// match, and that is the common case with a list already appearing
		// underneath: the feedback is on screen, so a bell would be noise.
		fmt.Fprint(e.screen, "\a")
		return
	}
	if len(matches) == 1 {
		e.replaceWord(typed, insert(matches[0]))
		if !strings.HasSuffix(matches[0], "/") {
			// A finished name closes the quote it was typed in, as bash closes it; a
			// directory leaves it open, so the next Tab goes on inside it.
			if quote != 0 {
				e.buffer.insert(quote)
			}
			e.buffer.insert(' ')
		}
		return
	}
	// Not "longer than the stem". Every match matches the stem, so the shared
	// prefix is never shorter than it -- but on Windows it can be the same length
	// and a different case, and that is worth inserting too: it puts the name on
	// the line as it is really spelled. busybox-w32 does the same, and says so:
	// "replace match prefix to allow for altered case" (libbb/lineedit.c:1531).
	if shared := longestSharedPrefix(matches); shared != stem {
		e.replaceWord(typed, insert(shared))
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
// expects a second Tab to show. The column layout is in complete_listing.go.
//
// A listing that would fill the screen asks first, which is bash's behaviour and
// for bash's reason: command completion reaches PATH, so `w` has a hundred and
// eighteen answers here and a bare Tab has two thousand. Printing those without
// asking scrolls the session away; refusing outright takes the decision from
// someone who may well want to look. Asking is the only one of the three that
// does neither.
func (e *lineEditor) listCandidates(matches []string, prompt string) {
	sortCandidates(matches)
	e.listCandidatesAsGiven(matches, prompt)
}

// listCandidatesAsGiven lists the choices in the order they come in, which -o nosort asks for.
func (e *lineEditor) listCandidatesAsGiven(matches []string, prompt string) {
	listing, rows := layoutCandidates(matches, e.columnsOrDefault())
	fmt.Fprintln(e.screen)
	if rows > listedRowLimit && !e.confirmLongListing(len(matches)) {
		fmt.Fprint(e.screen, prompt)
		e.resetDrawState()
		return
	}
	fmt.Fprint(e.screen, listing)
	fmt.Fprint(e.screen, prompt)
	e.resetDrawState()
}

// confirmLongListing asks before filling the screen, and reports the answer.
//
// The key is read from the same stream the editor is already reading, which is
// safe here because this runs inside the key loop rather than beside it. Anything
// other than y is no, and a stream that has ended is no -- a question nobody can
// answer must not print two thousand lines.
func (e *lineEditor) confirmLongListing(count int) bool {
	fmt.Fprintf(e.screen, "Display all %d possibilities? (y or n) ", count)
	answer, err := e.nextKey()
	fmt.Fprintln(e.screen)
	if err != nil {
		return false
	}
	return answer.kind == keyRune && (answer.value == 'y' || answer.value == 'Y')
}
