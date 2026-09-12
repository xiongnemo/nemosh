package applets

import (
	"strconv"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// The line-number gutter: whether the number on a row is the line on that row.
//
// Drawn on a frame rather than checked as a string, because the whole reason it lives
// inside the widget is that the scroll offset has to be read after the text area has
// drawn -- and only a frame exercises that ordering.

// drawGutterFrame lays out one buffer through the real widget and answers the screen.
func drawGutterFrame(t *testing.T, text string, width, height int) tcell.SimulationScreen {
	t.Helper()
	area := newHighlightedArea(nil)
	area.showLineNumbers(true)
	area.SetText(text, false)
	area.relex()
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

// gutterOf reads the leftmost columns of a row.
func gutterOf(t *testing.T, screen tcell.SimulationScreen, row, width int) string {
	t.Helper()
	var out strings.Builder
	for column := 0; column < width; column++ {
		text, _, _ := screen.Get(column, row)
		if text == "" {
			text = " "
		}
		out.WriteString(text)
	}
	return out.String()
}

// The numbers count from one and stop at the end of the buffer -- past it the gutter is
// blank rather than numbering rows that hold no line, which is what nano does.
func TestGutter_numbersTheLinesAndStopsAtTheEnd(t *testing.T) {
	screen := drawGutterFrame(t, "alpha\nbeta\ngamma\n", 40, 8)
	const gutter = 1 + gutterGap
	for row, want := range []string{"1 ", "2 ", "3 ", "  ", "  "} {
		if got := gutterOf(t, screen, row, gutter); got != want {
			t.Errorf("row %d has gutter %q, want %q", row, got, want)
		}
	}
	// And the text begins immediately after it.
	if got := gutterOf(t, screen, 0, gutter+5); got != "1 alpha" {
		t.Errorf("row 0 reads %q", got)
	}
}

// The width follows the line count, so a short file does not pay for a long one, and
// the numbers stay right-aligned when it changes.
func TestGutter_widthFollowsTheLineCount(t *testing.T) {
	for _, test := range []struct {
		lines int
		want  int
	}{
		{lines: 1, want: 1 + gutterGap},
		{lines: 9, want: 1 + gutterGap},
		{lines: 10, want: 2 + gutterGap},
		{lines: 99, want: 2 + gutterGap},
		{lines: 100, want: 3 + gutterGap},
		{lines: 1234, want: 4 + gutterGap},
	} {
		area := newHighlightedArea(nil)
		area.showLineNumbers(true)
		area.SetText(strings.Repeat("x\n", test.lines), false)
		area.relex()
		area.SetRect(0, 0, 80, 10)
		if got := area.gutterWidth(); got != test.want {
			t.Errorf("%d lines gives a gutter of %d, want %d", test.lines, got, test.want)
		}
		// The text area gets the rest, and no more.
		if _, _, textWidth, _ := area.TextArea.GetRect(); textWidth != 80-test.want {
			t.Errorf("%d lines leaves the text %d columns, want %d", test.lines, textWidth, 80-test.want)
		}
	}
}

// Right alignment: at a hundred lines the single digits are padded so the units line up,
// which is the whole reason to right-align rather than left.
func TestGutter_rightAlignsTheNumbers(t *testing.T) {
	screen := drawGutterFrame(t, strings.Repeat("x\n", 100), 40, 4)
	const gutter = 3 + gutterGap
	for row, want := range []string{"  1 ", "  2 ", "  3 ", "  4 "} {
		if got := gutterOf(t, screen, row, gutter); got != want {
			t.Errorf("row %d has gutter %q, want %q", row, got, want)
		}
	}
}

// Scrolled, the numbers follow -- which is the property the design is built around, and
// the one a separate widget beside the text could not guarantee.
func TestGutter_followsTheScrollOffset(t *testing.T) {
	const width, height = 40, 5
	area := newHighlightedArea(nil)
	area.showLineNumbers(true)
	var buffer strings.Builder
	for line := 1; line <= 200; line++ {
		buffer.WriteString("line " + strconv.Itoa(line) + "\n")
	}
	area.SetText(buffer.String(), false)
	area.relex()
	area.SetRect(0, 0, width, height)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(width, height)

	area.SetOffset(120, 0)
	area.Draw(screen)
	screen.Show()

	// Row 0 shows line 121, and the number matches the text on the same row -- asserted
	// together, because a gutter that is internally consistent but a row out from the
	// text is exactly the bug this design exists to prevent.
	const gutter = 3 + gutterGap
	for row := 0; row < height; row++ {
		whole := gutterOf(t, screen, row, width)
		number, text, _ := strings.Cut(strings.TrimSpace(whole), " ")
		if number != strconv.Itoa(121+row) {
			t.Errorf("row %d is numbered %q, want %d", row, number, 121+row)
		}
		if text != "line "+strconv.Itoa(121+row) {
			t.Errorf("row %d numbered %s holds %q", row, number, text)
		}
	}
}

// A terminal too narrow to afford it loses the gutter rather than the text.
func TestGutter_isDroppedWhenTheTextWouldHaveNoRoom(t *testing.T) {
	area := newHighlightedArea(nil)
	area.showLineNumbers(true)
	area.SetText(strings.Repeat("x\n", 1000), false)
	area.relex()

	// Wide enough: four digits and a gap, leaving well over the minimum.
	area.SetRect(0, 0, 60, 10)
	if got := area.gutterWidth(); got != 4+gutterGap {
		t.Errorf("at 60 columns the gutter is %d, want %d", got, 4+gutterGap)
	}
	// Not wide enough: the text would be left under the minimum, so the numbers go.
	area.SetRect(0, 0, 24, 10)
	if got := area.gutterWidth(); got != 0 {
		t.Errorf("at 24 columns the gutter is %d, want it dropped", got)
	}
	if _, _, textWidth, _ := area.TextArea.GetRect(); textWidth != 24 {
		t.Errorf("dropping the gutter left the text %d columns, want the whole 24", textWidth)
	}
}

// Off by default at the widget level, which is what keeps the highlighting tests --
// which assert colours at known columns -- honest.
func TestGutter_isOffUntilAskedFor(t *testing.T) {
	area := newHighlightedArea(nil)
	area.SetText("a\nb\n", false)
	area.relex()
	area.SetRect(0, 0, 40, 5)
	if got := area.gutterWidth(); got != 0 {
		t.Fatalf("a fresh widget has a gutter of %d, want none", got)
	}
	// But the editor turns it on, so a real session has one.
	view := newEditorView(&editorSession{name: "nano", path: "a.go", text: "a\nb\n"},
		editorKeyMapFor("nano"), nil)
	view.area.SetRect(0, 0, 80, 10)
	if got := view.area.gutterWidth(); got == 0 {
		t.Fatal("the editor's own text area has no gutter")
	}
}

// A file with no syntax still has line numbers: the lines are counted whether or not
// there is a language, which is why relex no longer returns early before counting them.
func TestGutter_worksWithoutASyntax(t *testing.T) {
	view := newEditorView(&editorSession{name: "nano", path: "notes.txt", text: "one\ntwo\n"},
		editorKeyMapFor("nano"), nil)
	if view.area.syntax != nil {
		t.Fatal("notes.txt matched a language; this test needs one that does not")
	}
	view.area.SetRect(0, 0, 80, 10)
	if got := view.area.gutterWidth(); got != 1+gutterGap {
		t.Fatalf("an unhighlighted file has a gutter of %d, want %d", got, 1+gutterGap)
	}
}

// Highlighting and the gutter are only ever run *together* by the editor, and until this
// test no case covered the combination: every colour-at-a-column assertion built a widget
// with the gutter off, and every gutter assertion built one with no syntax. The gutter
// shifts every text column, so the gap sat exactly where a mistake would land -- the same
// shape as the `^_` bug, where the tested configuration was not the shipped one.
//
// It passes, and for a reason rather than by luck: the colour pass reads the *text area's*
// inner rect, which SetRect has already moved, so the columns follow the text without
// either feature knowing the other exists. Asserted so it stays that way.
func TestGutter_highlightingStillLandsOnTheRightColumns(t *testing.T) {
	highlightSyntaxList()
	area := newHighlightedArea(highlightByName["go"])
	area.showLineNumbers(true)
	area.SetText("func main() { // note\n", false)
	area.relex()
	area.SetRect(0, 0, 40, 5)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(40, 5)
	area.Draw(screen)
	screen.Show()

	const gutter = 1 + gutterGap
	at := func(column int) tcell.Color {
		_, style, _ := screen.Get(column, 0)
		foreground, _, _ := style.Decompose()
		return foreground
	}
	if got, want := at(gutter+0), tcell.ColorFuchsia; got != want {
		t.Errorf("the keyword at column %d is %v, want %v", gutter+0, got, want)
	}
	if got, want := at(gutter+14), tcell.ColorGray; got != want {
		t.Errorf("the comment at column %d is %v, want %v", gutter+14, got, want)
	}
	if got := at(gutter + 5); got == tcell.ColorFuchsia {
		t.Errorf("the identifier at column %d is keyword-coloured, so the spans shifted", gutter+5)
	}
	// And no colour bleeds into the gutter itself.
	for column := 0; column < gutter; column++ {
		if got := at(column); got == tcell.ColorFuchsia {
			t.Errorf("gutter column %d got the keyword colour", column)
		}
	}
}
