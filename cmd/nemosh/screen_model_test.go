package main

import (
	"strings"
	"testing"
)

// screenModel is a terminal, as far as the line editor can tell.
//
// It exists because the editor's tests asserted the bytes it emitted rather
// than what those bytes do. Both defects a user hit were in the second: a
// prompt measured 11 columns instead of 2, and a wrapped line was rewritten
// over the wrong rows. An assertion on `\033[3C` cannot catch either, because
// the sequence is well-formed in both the working and the broken version --
// only the number is wrong. This is the same mistake as asserting that `times`
// printed the %dm%fs shape while it reported 215 years.
//
// It interprets exactly what the editor emits and nothing else. An unknown
// sequence is a test failure rather than a silent no-op, so the model cannot
// quietly diverge from the editor.
type screenModel struct {
	width int
	rows  [][]rune
	// attributes runs alongside rows, one entry per cell, holding the SGR
	// parameters in force when that cell was written.
	//
	// Tracked because the styling is not decoration to the tests: an underline
	// under the wrong word, or grey over text the user actually typed, is a
	// defect that byte assertions cannot see. `\033[4m` is well formed wherever
	// it appears, exactly as `\033[3C` was.
	attributes [][]string
	current    string
	row        int
	col        int
	t          *testing.T
}

func newScreenModel(t *testing.T, width int) *screenModel {
	t.Helper()
	return &screenModel{width: width, rows: [][]rune{{}}, attributes: [][]string{{}}, t: t}
}

// Write feeds the editor's output through the model.
func (s *screenModel) Write(p []byte) (int, error) {
	text := string(p)
	for index := 0; index < len(text); {
		switch {
		case text[index] == '\r':
			s.col = 0
			index++
		case text[index] == '\n':
			s.row++
			s.col = 0
			s.ensureRow(s.row)
			index++
		case text[index] == 0x1b:
			index += s.applyEscape(text[index:])
		default:
			r, size := decodeRuneAt(text, index)
			s.put(r)
			index += size
		}
	}
	return len(p), nil
}

// applyEscape interprets one sequence and returns how many bytes it consumed.
func (s *screenModel) applyEscape(text string) int {
	s.t.Helper()
	end := skipEscapeSequence(text, 0)
	sequence := text[:end]
	if !strings.HasPrefix(sequence, "\033[") {
		s.t.Fatalf("screen model saw a sequence it does not implement: %q", sequence)
	}
	body := sequence[2 : len(sequence)-1]
	final := sequence[len(sequence)-1]
	count := 1
	// A colour carries several parameters separated by semicolons and changes
	// nothing about layout, so only the first is parsed and only the movement
	// finals below use it.
	if first, _, _ := strings.Cut(body, ";"); first != "" {
		count = 0
		for _, digit := range first {
			if digit < '0' || digit > '9' {
				s.t.Fatalf("screen model saw a parameter it does not implement: %q", sequence)
			}
			count = count*10 + int(digit-'0')
		}
	}
	switch final {
	case 'C':
		s.col += count
	case 'D':
		s.col = max(0, s.col-count)
	case 'A':
		s.row = max(0, s.row-count)
	case 'B':
		s.row += count
		s.ensureRow(s.row)
	case 'J':
		// Erase from the cursor to the end of the display, which is the only
		// form the editor emits.
		s.rows[s.row] = s.rows[s.row][:min(s.col, len(s.rows[s.row]))]
		s.attributes[s.row] = s.attributes[s.row][:min(s.col, len(s.attributes[s.row]))]
		s.rows = s.rows[:s.row+1]
		s.attributes = s.attributes[:s.row+1]
	case 'K':
		// Erase from the cursor to the end of the *line*, which is what the
		// incremental search emits: its prompt changes width with every
		// keystroke, so the row is cleared rather than patched.
		s.rows[s.row] = s.rows[s.row][:min(s.col, len(s.rows[s.row]))]
		s.attributes[s.row] = s.attributes[s.row][:min(s.col, len(s.attributes[s.row]))]
	case 'H':
		s.row, s.col = 0, 0
	case 'm':
		// Styling changes nothing about layout, which is the whole point of the
		// defect this model exists to catch -- but it does change what a person
		// sees, so it is recorded rather than discarded. `0` and an empty body
		// both mean "back to plain".
		if body == "" || body == "0" {
			s.current = ""
			break
		}
		s.current = body
	default:
		s.t.Fatalf("screen model saw a final byte it does not implement: %q", sequence)
	}
	return end
}

func (s *screenModel) put(r rune) {
	width := runeColumns(r)
	if width == 0 {
		return
	}
	if s.width > 0 && s.col+width > s.width {
		s.row++
		s.col = 0
		s.ensureRow(s.row)
	}
	s.ensureRow(s.row)
	for len(s.rows[s.row]) < s.col {
		s.rows[s.row] = append(s.rows[s.row], ' ')
		s.attributes[s.row] = append(s.attributes[s.row], "")
	}
	if s.col < len(s.rows[s.row]) {
		s.rows[s.row] = s.rows[s.row][:s.col]
		s.attributes[s.row] = s.attributes[s.row][:s.col]
	}
	s.rows[s.row] = append(s.rows[s.row], r)
	s.attributes[s.row] = append(s.attributes[s.row], s.current)
	// A wide character occupies a second cell that holds nothing of its own.
	for filler := 1; filler < width; filler++ {
		s.rows[s.row] = append(s.rows[s.row], 0)
		s.attributes[s.row] = append(s.attributes[s.row], s.current)
	}
	s.col += width
}

func (s *screenModel) ensureRow(row int) {
	for len(s.rows) <= row {
		s.rows = append(s.rows, []rune{})
		s.attributes = append(s.attributes, []string{})
	}
}

// styleAt reports the SGR parameters in force at one cell, joined with `;` so a
// test can name it as it was written: "32", "32;4", "" for plain.
func (s *screenModel) styleAt(row, col int) string {
	if row >= len(s.attributes) || col >= len(s.attributes[row]) {
		return ""
	}
	return s.attributes[row][col]
}

// styledRun returns the text of the cells sharing the style at a starting cell,
// which is how a test asks "what exactly is underlined".
func (s *screenModel) styledRun(row, col int) string {
	want := s.styleAt(row, col)
	var run strings.Builder
	for index := col; index < len(s.rows[row]); index++ {
		if s.styleAt(row, index) != want {
			break
		}
		if s.rows[row][index] != 0 {
			run.WriteRune(s.rows[row][index])
		}
	}
	return run.String()
}

// text returns what a person would see on one row.
func (s *screenModel) text(row int) string {
	if row >= len(s.rows) {
		return ""
	}
	var visible strings.Builder
	for _, r := range s.rows[row] {
		if r != 0 {
			visible.WriteRune(r)
		}
	}
	return strings.TrimRight(visible.String(), " ")
}

// cursor is where the next character would land.
func (s *screenModel) cursor() (int, int) { return s.row, s.col }

func (s *screenModel) rowCount() int { return len(s.rows) }
