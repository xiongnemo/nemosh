// Package textgrid measures text in terminal cells and lays it out in columns.
//
// It exists because three places needed the same two answers and only one of them had them
// right. The line editor knew how many cells a rune draws in -- it has to, or the cursor lands
// in the wrong place after a CJK character -- and the completion listing used that to build a
// proper column-major grid. The `help` builtin word-wrapped at a hard-coded 68 measured in
// *bytes*, so one wide character skewed the line. And `ls` had no columns at all: one entry per
// line, always, with `-C` refused by name.
//
// The measuring code lived in `cmd/nemosh`, which is why the two applets could not use it. That
// is the whole reason this package is here rather than a fourth implementation being written.
package textgrid

import (
	"strings"
	"unicode"

	"golang.org/x/text/width"
)

// RuneCells is how many terminal cells a rune occupies.
//
// The wide test comes from the Unicode data rather than a table written out by hand. The hand
// table was missing U+231A WATCH and U+1F680 ROCKET, both of which conhost and Windows Terminal
// draw two cells wide, so the cursor drifted one cell for every one of them on the line -- found
// by printing a ruler into a real console, not by reading the table.
//
// EastAsianAmbiguous stays at one cell: Latin-1 punctuation, Greek and Cyrillic are drawn wide
// only in a CJK-locale terminal, and one cell is what wcwidth defaults to.
func RuneCells(r rune) int {
	switch {
	case r == 0:
		return 0
	case r < 0x20 || (r >= 0x7f && r < 0xa0):
		// A control character is never drawn as itself.
		return 0
	case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf):
		// A combining mark or format character attaches to the glyph before it and adds
		// no cell of its own. conhost disagrees -- it gives the mark a cell of its own --
		// and that is a divergence recorded rather than followed, since Unicode and every
		// other terminal, Windows Terminal included, say zero.
		return 0
	case isWide(r):
		return 2
	default:
		return 1
	}
}

// Cells is how many terminal cells a string occupies, which is not its length in bytes and not
// its length in runes either.
func Cells(text string) int {
	total := 0
	for _, r := range text {
		total += RuneCells(r)
	}
	return total
}

func isWide(r rune) bool {
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return true
	default:
		return false
	}
}

// Item is a grid entry whose printed form is wider in bytes than it is on screen: a filename
// wrapped in colour escapes prints as thirty bytes and occupies eight cells. Grid measures what
// it is given, so anything invisible has to be declared rather than guessed at.
type Item struct {
	Text  string
	Cells int
}

// Grid lays plain strings out in columns, measuring them itself.
func Grid(items []string, width int) ([]string, int) {
	measured := make([]Item, len(items))
	for index, item := range items {
		measured[index] = Item{Text: item, Cells: Cells(item)}
	}
	return GridOf(measured, width)
}

// GridOf lays out items that carry their own cell count, returning the lines and how many rows
// they took.
//
// Column-major -- down a column, then across -- which is what busybox and GNU both do, and what
// makes reading down a column alphabetical when the input is sorted. The field is the widest item
// plus two, in cells, and the last column on a row is not padded so a row that fills the terminal
// does not wrap into an empty one.
func GridOf(items []Item, width int) ([]string, int) {
	if len(items) == 0 {
		return nil, 0
	}
	field := 0
	for _, item := range items {
		field = max(field, item.Cells)
	}
	field += 2
	columns := max(width/field, 1)
	rows := (len(items) + columns - 1) / columns
	lines := make([]string, 0, rows)
	for row := range rows {
		var line strings.Builder
		for column := range columns {
			index := column*rows + row
			if index >= len(items) {
				break
			}
			line.WriteString(items[index].Text)
			if index+rows < len(items) {
				line.WriteString(strings.Repeat(" ", field-items[index].Cells))
			}
		}
		lines = append(lines, line.String())
	}
	return lines, rows
}
