package applets

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Whether the editor uses the width it has.
//
// Reported from a real terminal, and invisible to every test that existed -- because
// the harness runs at tview's default 80 columns, which is the one width the old code
// was correct at. That is the shape of this bug: a layout decided once, for a size
// nobody measured.
//
// Two things were wrong. The legend was laid out by `keys.footer(80)` exactly once at
// construction, so on a wider terminal the key hints sat in the leftmost 61 columns.
// And the title was twenty characters of text on an otherwise blank line, so nothing
// marked the top of the window as belonging to the editor at all.

// The layout responds to the width: more columns and fewer rows as the terminal grows.
//
// This function was always right. The bug was that the editor asked it for a layout at
// a hardcoded 80 exactly once, so a wider terminal kept the 80-column answer -- two
// rows of 61 characters -- and left everything to the right of them empty.
func TestEditorFooter_laysOutForTheWidthItIsGiven(t *testing.T) {
	keys := editorKeyMapFor("nano")
	for _, test := range []struct {
		width    int
		wantRows int
	}{
		// Five columns fit at 80, so seven labels need two rows.
		{width: 80, wantRows: 2},
		{width: 100, wantRows: 2},
		// Seven columns fit at 112, so one row holds them all.
		{width: 120, wantRows: 1},
		{width: 200, wantRows: 1},
		// And a narrow terminal stacks them rather than truncating.
		{width: 40, wantRows: 4},
		{width: 16, wantRows: 7},
	} {
		lines := keys.footer(test.width)
		if len(lines) != test.wantRows {
			t.Errorf("at width %d the legend is %d rows, want %d", test.width, len(lines), test.wantRows)
		}
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				t.Errorf("at width %d the legend has an empty row", test.width)
			}
			if len(line) > test.width && test.width >= footerColumnWidth*2 {
				t.Errorf("at width %d a legend row is %d characters and would be clipped",
					test.width, len(line))
			}
		}
	}
	// A wide terminal uses much more of it than the old fixed answer did: seven labels
	// in one row is a hundred and nine characters against the sixty-one it drew before,
	// whatever the width.
	widest := 0
	for _, line := range keys.footer(160) {
		if len(line) > widest {
			widest = len(line)
		}
	}
	if widest <= 61 {
		t.Errorf("at 160 columns the legend is %d characters wide, no better than the fixed 80-column answer", widest)
	}
}

// The view asks again when the width changes, which is the other half: a footer that
// can stretch is no use if it is only ever asked once.
func TestEditorView_relaysTheFooterWhenTheWidthChanges(t *testing.T) {
	view := newEditorView(&editorSession{name: "nano", path: "a.go"}, editorKeyMapFor("nano"), nil)
	first := view.legend.GetText(true)

	view.layoutFooter(200)
	wide := view.legend.GetText(true)
	if wide == first {
		t.Fatal("the legend did not change when the width did")
	}
	// Wider means the labels are further apart, so the row is longer.
	if len(firstLine(wide)) <= len(firstLine(first)) {
		t.Fatalf("at 200 columns the legend's first row is %d characters and at 80 it was %d",
			len(firstLine(wide)), len(firstLine(first)))
	}

	// Asked again at the same width, nothing is recomputed -- which is what makes it
	// safe to call from a draw function that runs every frame.
	view.layoutFooter(200)
	if again := view.legend.GetText(true); again != wide {
		t.Fatal("the legend changed when the width did not")
	}

	// And back down.
	view.layoutFooter(80)
	if narrow := view.legend.GetText(true); narrow != first {
		t.Fatalf("returning to 80 columns did not restore the original layout:\n  %q\n  %q", narrow, first)
	}
}

// The title is a bar across the whole width, which is what makes the top of the screen
// read as the editor's rather than as a stray line of text. Asserted on the drawn
// frame, because the background is the whole point and only a frame has one.
func TestEditorView_titleIsABarAcrossTheWidth(t *testing.T) {
	const width = 100
	view := newEditorView(&editorSession{name: "nano", path: "a.go", text: "package main\n"},
		editorKeyMapFor("nano"), nil)
	view.layout.SetRect(0, 0, width, 12)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(width, 12)
	view.layout.Draw(screen)
	screen.Show()

	// Every cell of the top row carries the title's background, including the ones
	// past the end of the text -- which is what "a bar" means.
	for _, column := range []int{0, 1, width / 2, width - 2, width - 1} {
		_, style, _ := screen.Get(column, 0)
		_, background, _ := style.Decompose()
		if background != tcell.ColorNavy {
			t.Errorf("column %d of the title row has background %v, want the title bar's", column, background)
		}
	}
	// And the row below it does not, so the bar is one row and not the whole screen.
	_, style, _ := screen.Get(width-1, 1)
	if _, background, _ := style.Decompose(); background == tcell.ColorNavy {
		t.Error("the row below the title also has the title background, so the bar is too tall")
	}
}

// The legend reaches the right-hand side of a wide terminal once drawn, which is the
// user-visible form of the bug: at 160 columns the old code drew key hints in the
// leftmost 61 and left ninety-nine blank.
func TestEditorView_legendReachesAcrossAWideTerminal(t *testing.T) {
	const width = 160
	view := newEditorView(&editorSession{name: "nano", path: "a.go", text: "x\n"},
		editorKeyMapFor("nano"), nil)
	view.layout.SetRect(0, 0, width, 12)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(width, 12)
	view.layout.Draw(screen)
	screen.Show()

	// The legend is the last two rows. Find the rightmost non-blank column on them.
	rightmost := -1
	for row := 10; row < 12; row++ {
		for column := 0; column < width; column++ {
			text, _, _ := screen.Get(column, row)
			if strings.TrimSpace(text) != "" && column > rightmost {
				rightmost = column
			}
		}
	}
	// Seven labels in sixteen-character columns is a hundred and nine characters, and
	// that is the honest ceiling: a compact legend can only be as wide as the labels it
	// has. What matters is that it is no longer the *fixed* sixty-one it drew at every
	// width, which is what the hardcoded 80 produced.
	if rightmost < 100 {
		t.Fatalf("on a %d-column terminal the legend stops at column %d, no better than the fixed 80-column answer",
			width, rightmost)
	}
}

// The text area itself fills the width, which is the largest part of the window and so
// the main thing "uses the terminal" means. It always did -- the Flex gives it the whole
// row -- and this asserts it so that the layout changes above cannot quietly take it
// away.
func TestEditorView_theTextAreaFillsTheWidth(t *testing.T) {
	const width = 140
	long := strings.Repeat("x", width+40)
	view := newEditorView(&editorSession{name: "nano", path: "a.txt", text: long + "\n"},
		editorKeyMapFor("nano"), nil)
	view.layout.SetRect(0, 0, width, 12)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(width, 12)
	view.layout.Draw(screen)
	screen.Show()

	// Row 1 is the first line of the buffer -- row 0 is the title bar. One line means a
	// one-digit gutter, so the text starts at column 2 and must reach the last column:
	// the gutter takes its columns from the left and gives up none on the right.
	const gutter = 1 + gutterGap
	if got, _, _ := screen.Get(0, 1); got != "1" {
		t.Errorf("column 0 of the first text row is %q, want the line number", got)
	}
	for _, column := range []int{gutter, width / 2, width - 1} {
		text, _, _ := screen.Get(column, 1)
		if text != "x" {
			t.Errorf("column %d of the first text row is %q, want the long line to reach it", column, text)
		}
	}
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}

// newEditorView is called with a nil application in these tests, which it tolerates
// because nothing here presses a key. Asserted so that a later change making the
// constructor use the application immediately fails here rather than in a panic
// somewhere else.
func TestEditorView_constructsWithoutAnApplication(t *testing.T) {
	view := newEditorView(&editorSession{name: "nano", path: "a.go"}, editorKeyMapFor("nano"), nil)
	if view.layout == nil || view.area == nil || view.legend == nil {
		t.Fatal("the view was not fully built")
	}
	var _ *tview.Flex = view.layout
}

// The legend's height follows its width, and settles on the second frame.
//
// Seven labels are two rows at 80 columns and one at 120, so the row the layout gives
// the legend has to change with the terminal. That resize happens inside a draw
// function, which means the frame that discovers the new width still uses the old
// height -- so the first frame after a resize leaves a blank row and the next one does
// not. One frame is invisible in a running editor, and asserting it here is what makes
// the lag a known quantity rather than a mystery blank line.
func TestEditorView_theLegendHeightSettlesAfterAResize(t *testing.T) {
	const width, height = 120, 12
	view := newEditorView(&editorSession{name: "nano", path: "a.go", text: "x\n"},
		editorKeyMapFor("nano"), nil)
	view.layout.SetRect(0, 0, width, height)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(width, height)

	// The first frame: the legend was laid out for 80 when the view was built, so it
	// has two rows allotted and one row of content.
	view.layout.Draw(screen)
	screen.Show()

	// The second: the width is known, the legend is one row, and the bottom row holds
	// it.
	view.layout.Draw(screen)
	screen.Show()

	bottom := rowText(screen, width, height-1)
	if !strings.Contains(bottom, "^O Write Out") || !strings.Contains(bottom, "^_ Go To Line") {
		t.Fatalf("the bottom row does not hold the whole legend: %q", bottom)
	}
	// And the row above it is text-area space again rather than a blank legend row.
	if above := rowText(screen, width, height-2); strings.Contains(above, "^X Exit") {
		t.Fatalf("the row above the legend still holds legend content: %q", above)
	}
}

func rowText(screen tcell.SimulationScreen, width, row int) string {
	var out strings.Builder
	for column := 0; column < width; column++ {
		text, _, _ := screen.Get(column, row)
		if text == "" {
			out.WriteByte(' ')
			continue
		}
		out.WriteString(text)
	}
	return strings.TrimRight(out.String(), " ")
}
