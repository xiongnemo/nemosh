package main

import (
	"unicode"

	"golang.org/x/text/width"
)

// runeColumns reports how many terminal cells a rune occupies.
//
// This is the number busybox's line editor gets wrong. Its backspace moves the
// cursor back one column and deletes one character, so a two-column CJK
// character leaves half of itself on screen. Editing counts characters; drawing
// counts columns; they are different numbers and the buffer keeps them apart.
//
// The wide test used to be a table written out by hand here, on the grounds that the East
// Asian Wide and Fullwidth ranges are stable and a module for a table that size would not pay
// for itself. The ranges are stable; the transcription was not. Measured in a real console:
// U+231A WATCH and U+1F680 ROCKET are both drawn two cells wide by conhost and by Windows
// Terminal, and both were absent from the table, so the cursor drifted by one cell for every
// one of them on the line. Unicode adds wide code points every year, so the table was going to
// keep being wrong in a way nobody would notice until it bit them.
//
// x/text/width answers from the Unicode data instead, for 22 KiB of binary -- measured, against
// 870 KiB of headroom under the size ceiling. It is the same authors and the same 3-Clause BSD
// as x/sys and x/term, which are already linked.
//
// EastAsianAmbiguous stays at one cell. Those code points -- Latin-1 punctuation, Greek,
// Cyrillic -- are drawn wide only in a CJK-locale terminal and narrow everywhere else, and
// one cell is what the hand table gave them and what wcwidth defaults to.
func runeColumns(r rune) int {
	switch {
	case r == 0:
		return 0
	case r < 0x20 || (r >= 0x7f && r < 0xa0):
		// A control character is never drawn as itself; the editor refuses to
		// buffer one, so this only guards a caller that measures raw text.
		return 0
	case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf):
		// A combining mark or a format character attaches to the glyph before
		// it and adds no cell of its own.
		return 0
	case isWideRune(r):
		return 2
	default:
		return 1
	}
}

// isWideRune reports whether a terminal draws the rune in two cells.
func isWideRune(r rune) bool {
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return true
	default:
		return false
	}
}

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
