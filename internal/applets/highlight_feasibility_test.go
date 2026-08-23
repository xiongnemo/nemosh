package applets

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Can a TextArea be syntax-highlighted at all?
//
// This is asked before any engine is written, because the answer decides whether
// there is a design to have. tview's TextArea has one `SetTextStyle` for the whole
// widget and no per-run styling, and `Draw` writes every cell with that one style --
// so highlighting has to happen *after* Draw, by re-colouring cells.
//
// That needs a map from screen cell to buffer position, and the two facts it rests on
// are what this test measures rather than assumes:
//
//   - `GetOffset()` is exported and gives the rows skipped at the top.
//   - With wrapping *off*, one screen row is one buffer line, so the mapping is
//     `bufferLine = offsetRow + screenRow`. With wrapping on it is a display row and
//     tview's line-start table is unexported, so there is no mapping at all.
//
// If this passes, the design is: wrap off, draw, then re-colour. If it fails, the
// only routes left are reimplementing the widget or forking tview.
func TestHighlightFeasibility_screenRowMapsToBufferLineWithoutWrap(t *testing.T) {
	const lines = 40
	var buffer strings.Builder
	for index := 0; index < lines; index++ {
		// Each line names itself, so a screen row can be checked against the line it
		// should be showing. Long enough to exceed the width, so that wrapping *on*
		// would visibly break the mapping.
		buffer.WriteString("line")
		buffer.WriteString(itoaSmall(index))
		buffer.WriteString(strings.Repeat("x", 120))
		buffer.WriteString("\n")
	}

	area := tview.NewTextArea()
	// The setting the whole approach depends on.
	area.SetWrap(false)
	area.SetText(buffer.String(), false)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	area.SetRect(0, 0, 80, 24)

	for _, offset := range []int{0, 5, 17} {
		area.SetOffset(offset, 0)
		area.Draw(screen)
		screen.Show()
		gotRow, gotColumn := area.GetOffset()
		if gotRow != offset || gotColumn != 0 {
			t.Fatalf("SetOffset(%d, 0) then GetOffset() = (%d, %d)", offset, gotRow, gotColumn)
		}
		// Screen row 0 must show buffer line `offset`, row 1 line `offset+1`, and so
		// on -- which is the mapping the whole design needs.
		for screenRow := 0; screenRow < 3; screenRow++ {
			want := "line" + itoaSmall(offset+screenRow)
			got := readSimulationRow(screen, screenRow, len(want))
			if got != want {
				t.Fatalf("with offset %d, screen row %d shows %q, want %q -- so the mapping does not hold",
					offset, screenRow, got, want)
			}
		}
	}

	// And the negative control: with wrapping *on*, the mapping breaks, which is why
	// the editor has to turn it off rather than this being a free choice.
	area.SetWrap(true)
	area.SetOffset(0, 0)
	area.Draw(screen)
	screen.Show()
	// Line 0 is 124 characters in an 80-column view, so it occupies two screen rows
	// and screen row 1 shows the *rest of line 0* rather than line 1.
	if got := readSimulationRow(screen, 1, 5); got == "line1" {
		t.Fatal("with wrapping on, screen row 1 showed line 1; the fixture is not long enough to prove anything")
	}
}

// A column offset shifts the text horizontally, which the re-colouring has to add
// back when it maps a cell to a position within the line.
func TestHighlightFeasibility_columnOffsetShiftsTheText(t *testing.T) {
	area := tview.NewTextArea()
	area.SetWrap(false)
	area.SetText("abcdefghij"+strings.Repeat("z", 100), false)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	area.SetRect(0, 0, 80, 24)

	area.SetOffset(0, 0)
	area.Draw(screen)
	screen.Show()
	if got := readSimulationRow(screen, 0, 4); got != "abcd" {
		t.Fatalf("with no column offset the row starts %q", got)
	}

	area.SetOffset(0, 4)
	area.Draw(screen)
	screen.Show()
	if got := readSimulationRow(screen, 0, 4); got != "efgh" {
		t.Fatalf("with column offset 4 the row starts %q, want the line shifted by four", got)
	}
}

func readSimulationRow(screen tcell.SimulationScreen, row, count int) string {
	cells, width, _ := screen.GetContents()
	var out strings.Builder
	for column := 0; column < count && column < width; column++ {
		runes := cells[row*width+column].Runes
		if len(runes) == 0 {
			out.WriteByte(' ')
			continue
		}
		out.WriteRune(runes[0])
	}
	return out.String()
}

func itoaSmall(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}
