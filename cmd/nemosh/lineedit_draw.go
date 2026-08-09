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
// Erasing uses the widest the line has ever been, so shrinking it -- a
// backspace over a two-column character -- does not leave the tail on screen.
func (e *lineEditor) redraw(prompt string) {
	// Measured with promptColumns, not textColumns: the prompt carries colour,
	// and every byte of an escape sequence but the ESC itself is printable.
	promptWidth := promptColumns(lastPromptLine(prompt))
	line := e.buffer.String()
	columns := e.buffer.columns()

	var out strings.Builder
	// To the start of the line, then past the prompt.
	out.WriteString("\r")
	if promptWidth > 0 {
		fmt.Fprintf(&out, "\033[%dC", promptWidth)
	}
	out.WriteString(line)
	// Erase whatever the previous, longer line left behind.
	if e.drawn > columns {
		out.WriteString(strings.Repeat(" ", e.drawn-columns))
	}
	// Back to the start again, then out to where the cursor belongs.
	out.WriteString("\r")
	if target := promptWidth + e.buffer.cursorColumns(); target > 0 {
		fmt.Fprintf(&out, "\033[%dC", target)
	}
	fmt.Fprint(e.screen, out.String())
	e.drawn = columns
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
	word := e.buffer.currentWord()
	var matches []string
	if completesCommand(e.buffer.currentWordPrefix()) {
		matches = completeCommand(word)
	} else {
		matches = completeFile(e.workingDirectory, word)
	}
	if len(matches) == 0 {
		return
	}
	if len(matches) == 1 {
		e.replaceWord(word, matches[0])
		if !strings.HasSuffix(matches[0], "/") {
			e.buffer.insert(' ')
		}
		return
	}
	shared := longestSharedPrefix(matches)
	if len(shared) > len(word) {
		e.replaceWord(word, shared)
	}
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
	e.drawn = 0
}
