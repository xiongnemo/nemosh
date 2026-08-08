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

// textColumns measures a whole string the same way.
func textColumns(text string) int {
	total := 0
	for _, r := range text {
		total += runeColumns(r)
	}
	return total
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
