package applets

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

// Drawing the highlighting, which tview's TextArea has no API for.
//
// The widget has one `SetTextStyle` for all of its text and writes every cell with it,
// so the only seam is *after* Draw: let the widget lay the text out -- wrapping,
// scrolling, cursor, selection, all of it -- and then re-colour the cells. The
// feasibility of that was measured before any of this was written; see
// highlight_feasibility_test.go, which also holds the negative control.
//
// Two facts make it work, and both are load-bearing:
//
//   - **Wrapping is off**, so one screen row is one buffer line and the mapping is
//     `line = rowOffset + row`. With wrapping on, a screen row is a *display* row and
//     tview's line-start table is unexported -- there is no mapping at all. That is why
//     the editor turns wrapping off rather than it being a preference.
//   - **The advance is uniseg's**, the same package and the same rule tview uses:
//     cluster width from `boundaries >> uniseg.ShiftWidth`, except a tab, which is a
//     flat `tview.TabSize` rather than an alignment to the next stop. Matching it
//     exactly is what stops the colours drifting from the characters -- and uniseg is
//     already linked, because tview uses it, so this costs a line in go.mod and no
//     bytes.

// highlightedArea is a TextArea that colours itself.
type highlightedArea struct {
	*tview.TextArea
	syntax *highlightSyntax
	// lines and spans are the lexed buffer. Recomputed when the text changes and not
	// when it scrolls, because scrolling cannot change what a line means.
	lines []string
	spans [][]highlightSpan
}

func newHighlightedArea(syntax *highlightSyntax) *highlightedArea {
	area := &highlightedArea{TextArea: tview.NewTextArea(), syntax: syntax}
	// Required, not chosen: see the file comment. It is also what micro does, and what
	// a reader of code generally wants -- a wrapped line of code is harder to read
	// than a scrolled one.
	area.SetWrap(false)
	return area
}

// relex re-reads the buffer. Called when the text changes.
func (a *highlightedArea) relex() {
	if a.syntax == nil {
		return
	}
	a.lines = editorLines(a.GetText())
	a.spans = highlightBuffer(a.syntax, a.lines)
}

// Draw lays the text out through the widget, then re-colours what it drew.
func (a *highlightedArea) Draw(screen tcell.Screen) {
	a.TextArea.Draw(screen)
	if a.syntax == nil || len(a.spans) == 0 {
		return
	}
	x, y, width, height := a.GetInnerRect()
	rowOffset, columnOffset := a.GetOffset()
	plain := a.GetTextStyle()
	for row := 0; row < height; row++ {
		index := rowOffset + row
		if index >= len(a.lines) || index >= len(a.spans) {
			break
		}
		colourLine(screen, a.lines[index], a.spans[index],
			x, y+row, width, columnOffset, plain)
	}
}

// colourLine walks one line's grapheme clusters and re-colours the cells of any that
// fall inside a span.
//
// The `style != plain` skip is what keeps the selection and the cursor visible without
// this code knowing anything about either: the widget drew those cells with a different
// style, so they are already owned and are left alone. Reading the cell back rather
// than asking the widget what is selected is both simpler and correct for whatever else
// tview may decide to style later.
func colourLine(screen tcell.Screen, line string, spans []highlightSpan,
	x, y, width, columnOffset int, plain tcell.Style) {
	if len(spans) == 0 {
		return
	}
	offset, column, next := 0, 0, 0
	state := -1
	rest := line
	for len(rest) > 0 {
		cluster, remaining, boundaries, newState := uniseg.StepString(rest, state)
		clusterWidth := boundaries >> uniseg.ShiftWidth
		if cluster == "\t" {
			clusterWidth = tview.TabSize
		}
		// Advance the span cursor to the one covering this byte. Spans are ordered and
		// disjoint -- asserted by the engine's tests -- so this never goes backwards.
		for next < len(spans) && spans[next].end <= offset {
			next++
		}
		if next < len(spans) && offset >= spans[next].start {
			if group := spans[next].group; group != groupNone {
				paintCluster(screen, group,
					x, y, width, column-columnOffset, clusterWidth, plain)
			}
		}
		offset += len(cluster)
		column += clusterWidth
		rest, state = remaining, newState
		if column-columnOffset > width {
			break
		}
	}
}

// paintCluster re-writes one cluster's cells in the group's style.
func paintCluster(screen tcell.Screen, group highlightGroup,
	x, y, width, column, clusterWidth int, plain tcell.Style) {
	if column < 0 || column >= width || clusterWidth <= 0 {
		return
	}
	// Get and Put rather than GetContent and SetContent: the older pair is deprecated
	// in this tcell, and Put takes the grapheme as a string, which is exactly what the
	// uniseg walk already has.
	drawn, style, _ := screen.Get(x+column, y)
	if style != plain {
		// The widget drew this cell as something else -- selected text, or the cursor.
		// Whatever it is, it outranks a colour, and reading the cell back means this
		// code needs to know nothing about either.
		return
	}
	if drawn == "" {
		// Nothing was drawn here, which happens past the end of a clipped line.
		return
	}
	screen.Put(x+column, y, drawn, highlightStyleFor(group, plain))
}

// highlightStyleFor is the palette.
//
// Foreground only, and from the terminal's own sixteen colours rather than from a
// twenty-four-bit scheme. Both are deliberate: keeping the background means the
// editor still sits inside whatever theme the terminal has, and using the named
// colours means a user who has re-themed their terminal gets *their* green rather than
// a green chosen here that may be unreadable against their background.
func highlightStyleFor(group highlightGroup, plain tcell.Style) tcell.Style {
	switch group {
	case groupComment:
		// Grey rather than a colour: a comment should recede, which is the one thing
		// every scheme agrees on.
		return plain.Foreground(tcell.ColorGray)
	case groupString:
		return plain.Foreground(tcell.ColorGreen)
	case groupKeyword:
		return plain.Foreground(tcell.ColorFuchsia)
	case groupNumber:
		return plain.Foreground(tcell.ColorRed)
	case groupType:
		return plain.Foreground(tcell.ColorAqua)
	case groupSymbol:
		// Barely marked. Colouring every bracket and comma is noise, but leaving them
		// identical to identifiers loses the shape of the code, so they get the
		// dimmest distinction available.
		return plain.Foreground(tcell.ColorSilver)
	}
	return plain
}
