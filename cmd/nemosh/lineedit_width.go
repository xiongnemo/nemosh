package main

import "github.com/xiongnemo/nemosh/internal/textgrid"

// runeColumns is how many terminal cells a rune occupies.
//
// The measuring lives in internal/textgrid now, because `ls`, `wc -L` and the `help` builtin
// need the same answer and could not reach it here -- see that package for why the hand-written
// wide table was replaced by the Unicode data. This stays as a name because the editor asks the
// question on every keystroke and `runeColumns` reads better at those call sites.
func runeColumns(r rune) int { return textgrid.RuneCells(r) }

// textColumns is how many cells a string occupies.
func textColumns(text string) int { return textgrid.Cells(text) }

// promptColumns measures what a prompt draws, skipping the escape sequences
// that colour it.
//
// textColumns cannot be used here: only the ESC that introduces a sequence is a
// control character, so `[1;31m` counts as six visible cells. A coloured `$ `
// measured 11 instead of 2, and since the editor places the cursor by moving it
// that far right of the line start, every keystroke landed nine cells past the
// prompt.
func promptColumns(prompt string) int {
	total := 0
	for index := 0; index < len(prompt); index++ {
		if prompt[index] != 0x1b {
			r, size := decodeRuneAt(prompt, index)
			total += runeColumns(r)
			index += size - 1
			continue
		}
		index = skipEscapeSequence(prompt, index) - 1
	}
	return total
}

// skipEscapeSequence returns the index just past the sequence starting at the
// ESC at index. A CSI runs to a final byte in @ through ~; anything else is
// two bytes. An unterminated sequence consumes the rest, which is right: it
// draws nothing either.
func skipEscapeSequence(text string, index int) int {
	index++
	if index >= len(text) {
		return index
	}
	if text[index] != '[' && text[index] != ']' {
		return index + 1
	}
	introducer := text[index]
	for index++; index < len(text); index++ {
		if introducer == '[' && text[index] >= '@' && text[index] <= '~' {
			return index + 1
		}
		// An OSC runs to BEL or ST; a title-setting prompt uses one.
		if introducer == ']' && (text[index] == 0x07 || text[index] == 0x1b) {
			return index + 1
		}
	}
	return len(text)
}

func decodeRuneAt(text string, index int) (rune, int) {
	for size := 1; size <= 4 && index+size <= len(text); size++ {
		candidate := text[index : index+size]
		for _, r := range candidate {
			if size == len(string(r)) {
				return r, size
			}
			break
		}
	}
	return rune(text[index]), 1
}
