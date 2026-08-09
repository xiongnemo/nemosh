package main

import "unicode"

// runeColumns reports how many terminal cells a rune occupies.
//
// This is the number busybox's line editor gets wrong. Its backspace moves the
// cursor back one column and deletes one character, so a two-column CJK
// character leaves half of itself on screen. Editing counts characters; drawing
// counts columns; they are different numbers and the buffer keeps them apart.
//
// No dependency for this: the East Asian Wide and Fullwidth ranges are stable,
// and pulling in a module for a table this size would be the project's first
// runtime dependency (AGENTS.md, Builds And Packaging).
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

// wideRanges are the Unicode East Asian Wide (W) and Fullwidth (F) blocks that
// a terminal draws in two cells.
var wideRanges = [...][2]rune{
	{0x1100, 0x115F},   // Hangul Jamo initial consonants
	{0x2E80, 0x303E},   // CJK radicals, Kangxi, CJK symbols and punctuation
	{0x3041, 0x33FF},   // Hiragana, Katakana, Bopomofo, Hangul Compatibility
	{0x3400, 0x4DBF},   // CJK Unified Ideographs Extension A
	{0x4E00, 0x9FFF},   // CJK Unified Ideographs
	{0xA000, 0xA4CF},   // Yi
	{0xA960, 0xA97F},   // Hangul Jamo Extended-A
	{0xAC00, 0xD7A3},   // Hangul syllables
	{0xF900, 0xFAFF},   // CJK Compatibility Ideographs
	{0xFE10, 0xFE19},   // Vertical forms
	{0xFE30, 0xFE6F},   // CJK Compatibility Forms, small form variants
	{0xFF00, 0xFF60},   // Fullwidth ASCII variants
	{0xFFE0, 0xFFE6},   // Fullwidth signs
	{0x1F300, 0x1F64F}, // Miscellaneous symbols and pictographs, emoticons
	{0x1F900, 0x1F9FF}, // Supplemental symbols and pictographs
	{0x20000, 0x2FFFD}, // CJK Extension B and beyond
	{0x30000, 0x3FFFD},
}

func isWideRune(r rune) bool {
	for _, span := range wideRanges {
		if r < span[0] {
			// The table is ascending, so nothing later can match.
			return false
		}
		if r <= span[1] {
			return true
		}
	}
	return false
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
