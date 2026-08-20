package textgrid_test

import (
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/textgrid"
)

func TestCells(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{text: "abcd", want: 4},
		{text: "中文", want: 4},
		{text: "中a文", want: 5},
		// Both of these were missing from the hand-written table this replaced, and both
		// are drawn two cells wide by conhost and by Windows Terminal.
		{text: "⌚🚀", want: 4},
		{text: "é", want: 1},
		{text: "", want: 0},
	}
	for _, test := range tests {
		if got := textgrid.Cells(test.text); got != test.want {
			t.Fatalf("Cells(%q) = %d, want %d", test.text, got, test.want)
		}
	}
}

// The layout busybox produces for twenty names in eighty columns, measured with `ls -C`:
// column-major, a field of the widest name plus two, and no padding after the last column.
func TestGrid_matchesTheMeasuredLayout(t *testing.T) {
	names := []string{
		"alpha", "beta", "delta", "epsilon", "eta", "gamma", "iota", "kappa", "lambda", "mu",
		"nu", "omicron", "pi", "rho", "sigma", "tau", "theta", "upsilon", "xi", "zeta",
	}

	// When
	lines, rows := textgrid.Grid(names, 80)

	// Then
	want := []string{
		"alpha    epsilon  iota     mu       pi       tau      xi",
		"beta     eta      kappa    nu       rho      theta    zeta",
		"delta    gamma    lambda   omicron  sigma    upsilon",
	}
	if rows != len(want) {
		t.Fatalf("rows = %d, want %d", rows, len(want))
	}
	for index, line := range lines {
		if line != want[index] {
			t.Fatalf("line %d = %q, want %q", index, line, want[index])
		}
	}
}

// The same names in forty columns, also measured from busybox.
func TestGrid_narrowerWidth(t *testing.T) {
	names := []string{
		"alpha", "beta", "delta", "epsilon", "eta", "gamma", "iota", "kappa", "lambda", "mu",
		"nu", "omicron", "pi", "rho", "sigma", "tau", "theta", "upsilon", "xi", "zeta",
	}

	// When
	lines, _ := textgrid.Grid(names, 40)

	// Then
	if want := "alpha    gamma    nu       tau"; lines[0] != want {
		t.Fatalf("first line = %q, want %q", lines[0], want)
	}
	if want := "eta      mu       sigma    zeta"; lines[len(lines)-1] != want {
		t.Fatalf("last line = %q, want %q", lines[len(lines)-1], want)
	}
}

// A row that fills the width must not be padded past it, or the terminal wraps it into an
// empty row and the listing takes twice the height.
func TestGrid_doesNotPadTheLastColumn(t *testing.T) {
	// When
	lines, _ := textgrid.Grid([]string{"aaa", "bbb"}, 10)

	// Then
	for _, line := range lines {
		if strings.HasSuffix(line, " ") {
			t.Fatalf("line %q ends in padding", line)
		}
	}
}

// A wide name has to widen the field by cells, not by runes: two CJK characters are four
// columns, and a grid that counted them as two would leave every later column short.
func TestGrid_measuresWideNamesInCells(t *testing.T) {
	// When
	lines, _ := textgrid.Grid([]string{"中文", "ab", "cd"}, 80)

	// Then -- the field is 4 + 2, so `ab` starts at column 6.
	if want := "中文  ab    cd"; lines[0] != want {
		t.Fatalf("line = %q, want %q", lines[0], want)
	}
}

// GridOf exists for the caller that has already painted its text: a name wrapped in colour
// escapes is bytes that occupy no cells, and measuring the printed form would shift every
// column after the first coloured entry.
func TestGridOf_measuresTheDeclaredWidth(t *testing.T) {
	items := []textgrid.Item{
		{Text: "\x1b[34mdir\x1b[0m", Cells: 3},
		{Text: "file", Cells: 4},
	}

	// When
	lines, _ := textgrid.GridOf(items, 80)

	// Then -- the field is 4 + 2, so the escapes contribute nothing to the padding.
	if want := "\x1b[34mdir\x1b[0m   file"; lines[0] != want {
		t.Fatalf("line = %q, want %q", lines[0], want)
	}
}

func TestGrid_empty(t *testing.T) {
	if lines, rows := textgrid.Grid(nil, 80); lines != nil || rows != 0 {
		t.Fatalf("Grid(nil) = %q, %d, want nothing", lines, rows)
	}
}
