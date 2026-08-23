package applets

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// Does the colour reach the screen?
//
// Everything else about highlighting is testable on strings; this is the part that
// only a drawn frame can answer, because it depends on tview's layout agreeing with
// this code's idea of where each character went. A mismatch here is the failure that
// looks like the colours have slipped a column.
//
// The screen is read back with `Get`, which answers the style as well as the text, so
// the assertions are about *which cells got which colour* rather than about a rune.

// styleAt is the foreground colour drawn at a cell.
func styleAt(t *testing.T, screen tcell.SimulationScreen, x, y int) tcell.Color {
	t.Helper()
	_, style, _ := screen.Get(x, y)
	foreground, _, _ := style.Decompose()
	return foreground
}

// drawHighlighted lays out one buffer through the real widget and answers the screen.
func drawHighlighted(t *testing.T, syntax *highlightSyntax, text string, width, height int) tcell.SimulationScreen {
	t.Helper()
	area := newHighlightedArea(syntax)
	area.SetText(text, false)
	area.relex()
	// No border and no label, so the inner rect is the whole thing and a cell's column
	// is its column.
	area.SetRect(0, 0, width, height)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(width, height)
	area.Draw(screen)
	screen.Show()
	return screen
}

// The basic claim: a keyword is one colour, a comment another, and the plain text
// neither.
func TestHighlightedArea_drawsGroupsInDifferentColours(t *testing.T) {
	highlightSyntaxList()
	screen := drawHighlighted(t, highlightByName["go"], "func main() { // note\n", 40, 5)

	keyword := styleAt(t, screen, 0, 0)  // the `f` of func
	plain := styleAt(t, screen, 5, 0)    // the `m` of main
	comment := styleAt(t, screen, 14, 0) // the first `/`
	if keyword == plain {
		t.Errorf("the keyword and the identifier are the same colour (%v)", keyword)
	}
	if comment == plain {
		t.Errorf("the comment and the identifier are the same colour (%v)", comment)
	}
	if keyword == comment {
		t.Errorf("the keyword and the comment are the same colour (%v)", keyword)
	}
	// And the keyword is the colour the palette says, which is what stops this test
	// passing on any three distinct colours.
	if want := tcell.ColorFuchsia; keyword != want {
		t.Errorf("the keyword is %v, want %v", keyword, want)
	}
	if want := tcell.ColorGray; comment != want {
		t.Errorf("the comment is %v, want %v", comment, want)
	}
}

// The alignment claim, and the one that would break silently: a double-width character
// earlier in the line must shift everything after it by two cells, not one.
//
// This is why the advance uses uniseg and tview's own tab rule rather than counting
// runes. A CJK comment above a keyword is an ordinary thing in a file on this machine.
func TestHighlightedArea_alignsAfterDoubleWidthCharacters(t *testing.T) {
	highlightSyntaxList()
	// `x := "路径"` then a keyword. The two CJK characters take four cells, so the
	// keyword begins at column 12 and not column 10.
	line := `x := "路径" return`
	screen := drawHighlighted(t, highlightByName["go"], line+"\n", 40, 5)

	// Where `return` begins, counted in *cells* rather than in characters -- which is
	// the whole point: the two CJK characters take four cells between them, so the
	// keyword is at column 12 and a rune count would look for it at column 10.
	at := strings.Index(line, "return")
	column := 0
	for _, r := range line[:at] {
		column += runeCellsForTest(r)
	}
	if column != 12 {
		t.Fatalf("the fixture puts the keyword at cell %d; this test is about it being at 12", column)
	}
	if got, want := styleAt(t, screen, column, 0), tcell.ColorFuchsia; got != want {
		t.Fatalf("the keyword after two CJK characters is %v at column %d, want %v -- the colours have slipped",
			got, column, want)
	}
	// And the cell before it is not the keyword colour, which is what says the span
	// did not start early.
	if got := styleAt(t, screen, column-1, 0); got == tcell.ColorFuchsia {
		t.Errorf("the cell before the keyword is also keyword-coloured, so the span starts a column early")
	}
}

// A tab is four cells in tview, flatly, rather than an alignment to the next stop. If
// this code assumed otherwise the colours would slip on every indented line -- which
// is every line of real code.
func TestHighlightedArea_alignsAfterTabs(t *testing.T) {
	highlightSyntaxList()
	screen := drawHighlighted(t, highlightByName["go"], "\t\treturn 1\n", 40, 5)
	// Two tabs at four cells each, so `return` begins at column 8.
	if got, want := styleAt(t, screen, 8, 0), tcell.ColorFuchsia; got != want {
		t.Fatalf("after two tabs the keyword is %v at column 8, want %v", got, want)
	}
}

// The column offset: a horizontally scrolled line must still be coloured correctly,
// which matters because wrapping is off and scrolling sideways is the only way to see
// a long line.
func TestHighlightedArea_honoursTheColumnOffset(t *testing.T) {
	highlightSyntaxList()
	area := newHighlightedArea(highlightByName["go"])
	area.SetText(strings.Repeat("x", 20)+" return 1\n", false)
	area.relex()
	area.SetRect(0, 0, 40, 5)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(40, 5)

	// `return` begins at column 21 unscrolled, so with the view scrolled ten columns
	// it should be drawn at column 11.
	area.SetOffset(0, 10)
	area.Draw(screen)
	screen.Show()
	if got, want := styleAt(t, screen, 11, 0), tcell.ColorFuchsia; got != want {
		t.Fatalf("with a column offset of 10 the keyword is %v at column 11, want %v", got, want)
	}
}

// A region that spans lines is coloured on every line it covers, including the ones
// with nothing else on them -- which is the visible payoff of lexing the buffer from
// the top rather than the viewport.
func TestHighlightedArea_coloursEveryLineOfABlockComment(t *testing.T) {
	highlightSyntaxList()
	screen := drawHighlighted(t, highlightByName["go"],
		"func a() {\n/* one\ntwo\nthree */\nfunc b() {\n", 40, 8)

	for row := 1; row <= 3; row++ {
		if got, want := styleAt(t, screen, 0, row), tcell.ColorGray; got != want {
			t.Errorf("row %d of the block comment is %v, want %v", row, got, want)
		}
	}
	// And the line after it is code again.
	if got := styleAt(t, screen, 0, 4); got != tcell.ColorFuchsia {
		t.Errorf("the line after the block comment is %v, want the keyword colour", got)
	}
}

// With no syntax -- a file whose name matched nothing -- every cell keeps the plain
// style, and nothing is re-coloured.
func TestHighlightedArea_noSyntaxLeavesEveryCellAlone(t *testing.T) {
	screen := drawHighlighted(t, nil, "func main() { // note\n", 40, 5)
	plain := styleAt(t, screen, 5, 0)
	for _, column := range []int{0, 1, 2, 3, 14, 15, 20} {
		if got := styleAt(t, screen, column, 0); got != plain {
			t.Errorf("column %d is %v with no syntax, want the plain %v", column, got, plain)
		}
	}
}

// Wrapping is off, which the whole approach depends on. Asserted on the widget rather
// than described in a comment, because a later change that turned it back on would
// break the colours and nothing else.
func TestHighlightedArea_wrappingIsOff(t *testing.T) {
	area := newHighlightedArea(nil)
	area.SetText(strings.Repeat("x", 200), false)
	area.SetRect(0, 0, 20, 5)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(20, 5)
	area.Draw(screen)
	screen.Show()
	// With wrapping on, row 1 would hold the continuation of row 0. With it off, row 1
	// is empty.
	if text, _, _ := screen.Get(0, 1); strings.TrimSpace(text) != "" {
		t.Fatalf("row 1 holds %q, so wrapping is on and the row-to-line mapping is gone", text)
	}
}

// runeCellsForTest is the test's own width count, deliberately not the production one:
// asserting the render against the same function it uses would prove only that the
// function equals itself.
func runeCellsForTest(r rune) int {
	switch {
	case r >= 0x1100 && r <= 0x115f, // Hangul Jamo
		r >= 0x2e80 && r <= 0xa4cf, // CJK radicals through Yi
		r >= 0xac00 && r <= 0xd7a3, // Hangul syllables
		r >= 0xf900 && r <= 0xfaff, // CJK compatibility
		r >= 0xff00 && r <= 0xff60, // fullwidth forms
		r >= 0xffe0 && r <= 0xffe6:
		return 2
	}
	return 1
}
