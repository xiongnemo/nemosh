package textgrid

import (
	"testing"

	"golang.org/x/text/width"
)

// The generated table says of every rune what x/text/width says, wide or fullwidth or neither:
// the table is that package's answer, in a few hundred bytes where the package linked 357 KB.
// Regenerate with `go generate ./internal/textgrid` when x/text's Unicode version moves.
func TestWideTable_agreesWithXTextWidthForEveryRune(t *testing.T) {
	for r := rune(0); r <= 0x10FFFF; r++ {
		kind := width.LookupRune(r).Kind()
		want := kind == width.EastAsianWide || kind == width.EastAsianFullwidth
		if got := isWide(r); got != want {
			t.Fatalf("U+%04X: the table says wide %v, x/text/width %v", r, got, want)
		}
	}
	for index := 1; index < len(wideRanges); index++ {
		if wideRanges[index][0] <= wideRanges[index-1][1]+1 {
			t.Fatalf("ranges %d and %d touch or overlap: %X, %X", index-1, index, wideRanges[index-1], wideRanges[index])
		}
	}
}
