package applets

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// less -R draws colour, as GNU's does: an SGR sequence, ESC [ ... m, sets the style of what
// follows -- bold, dim, italic, underline, blink, reverse, the sixteen colours, the 256 and RGB
// ones, and the resets -- and takes no column. Any other escape is drawn as the controls it is
// made of. A search matches the text without its colours, so a word split by a colour change is
// found and highlighted. It took the colour out, as busybox's -R does, where GNU's shows it:
// `git log --color | less -R` was monochrome.
func TestLess_rawDrawsColour(t *testing.T) {
	pager, screen := newTestPager(t, []string{
		"\x1b[31mred\x1b[0m plain \x1b[1;4;38;5;208mx\x1b[m",
		"\x1b[38;2;10;20;30;48;5;4mrgb\x1b[39;49m d \x1b[7mr\x1b[27m n",
		"\x1b]0;t\x07z",
	}, 40, 5)
	pager.settings.raw = true

	// When
	pager.draw()

	// Then
	for _, test := range []struct {
		column, row int
		text        string
		fg, bg      tcell.Color
		attrs       tcell.AttrMask
	}{
		{0, 0, "r", tcell.PaletteColor(1), tcell.ColorDefault, 0},
		{2, 0, "d", tcell.PaletteColor(1), tcell.ColorDefault, 0},
		{3, 0, " ", tcell.ColorDefault, tcell.ColorDefault, 0},
		{10, 0, "x", tcell.PaletteColor(208), tcell.ColorDefault, tcell.AttrBold | tcell.AttrUnderline},
		{0, 1, "r", tcell.NewRGBColor(10, 20, 30), tcell.PaletteColor(4), 0},
		{4, 1, "d", tcell.ColorDefault, tcell.ColorDefault, 0},
		{6, 1, "r", tcell.ColorDefault, tcell.ColorDefault, tcell.AttrReverse},
		{8, 1, "n", tcell.ColorDefault, tcell.ColorDefault, 0},
	} {
		text, style, _ := screen.Get(test.column, test.row)
		fg, bg, attrs := style.Decompose()
		if text != test.text || fg != test.fg || bg != test.bg || attrs&^tcell.AttrInvalid != test.attrs {
			t.Errorf("cell %d,%d = %q fg %v bg %v attrs %v; want %q fg %v bg %v attrs %v\n%q",
				test.column, test.row, text, fg, bg, attrs, test.text, test.fg, test.bg, test.attrs, rowOf(t, screen, test.row, 40))
		}
	}
	// An escape that is no colour is the controls it is made of, as without -R.
	if row := rowOf(t, screen, 2, 40); row != "[]0;tGz" {
		t.Errorf("an OSC sequence under -R is drawn as %q, want [ ]0;t G z", row)
	}

	// A search sees the words, not the sequences between their letters.
	pager.lines = []string{"one", "\x1b[31mre\x1b[1md\x1b[0m"}
	pager.pattern = "red"
	pager.runSearch(true, 0)
	if pager.message != "" {
		t.Fatalf("search for red over a coloured red said %q", pager.message)
	}
	pager.draw()
	for column := range 3 {
		if _, style, _ := screen.Get(column, 1); !hasReverse(style) {
			t.Errorf("column %d of the match is not highlighted: %q", column, rowOf(t, screen, 1, 40))
		}
	}
	if _, style, _ := screen.Get(0, 0); hasReverse(style) {
		t.Error("the line that does not match is highlighted")
	}
}

// Without a terminal, -R copies its input as it is, colours and all, as GNU's less copies it.
func TestLessWithoutATerminal_rawKeepsColour(t *testing.T) {
	got, _, status := runApplet(t, "less", []string{"-R"}, "\x1b[31mred\x1b[0m\n")
	if got != "\x1b[31mred\x1b[0m\n" || status != 0 {
		t.Fatalf("less -R into a pipe = %q, status %d", got, status)
	}
}

// lessWithoutSGR takes out exactly the sequences -R draws as colour, and nothing else.
func TestLess_withoutSGRTakesOutWhatIsDrawnAsColour(t *testing.T) {
	for _, test := range []struct{ in, want string }{
		{"\x1b[31mred\x1b[0m", "red"},
		{"\x1b[1;32mbold green\x1b[m plain", "bold green plain"},
		{"no colour", "no colour"},
		{"\x1b]0;title\x07x", "\x1b]0;title\x07x"},
		{"\x1b[2Jclear", "\x1b[2Jclear"},
	} {
		if got := lessWithoutSGR(test.in); got != test.want {
			t.Errorf("lessWithoutSGR(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}
