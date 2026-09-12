package applets

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// less.
//
// Two halves, tested two ways. The `cat` path -- what happens when there is nowhere to page
// to -- goes through the applet, because that is the path a script takes and the one that
// has to be right. The pager goes through a simulation screen, the same device the editor's
// tests use, because a scrolling rule is only worth anything if you can see what is on the
// screen after it.

func TestLessWithoutATerminal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// With nowhere to page to it is cat, which is what makes `less f | head` safe.
	got, stderr, status := runApplet(t, "less", []string{path}, "")
	if got != "alpha\nbeta\ngamma\n" || stderr != "" || status != 0 {
		t.Fatalf("got %q stderr %q status %d", got, stderr, status)
	}
	// -N is still honoured there, because a script may have wanted the numbers rather
	// than the paging.
	numbered, _, _ := runApplet(t, "less", []string{"-N", path}, "")
	if numbered != "     1  alpha\n     2  beta\n     3  gamma\n" {
		t.Fatalf("-N gave %q", numbered)
	}
	// Standard input when there is no operand, and `-` for it among operands.
	piped, _, _ := runApplet(t, "less", nil, "one\ntwo\n")
	if piped != "one\ntwo\n" {
		t.Fatalf("from stdin: %q", piped)
	}
	// Several files are concatenated.
	other := filepath.Join(dir, "g.txt")
	if err := os.WriteFile(other, []byte("delta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	both, _, _ := runApplet(t, "less", []string{path, other}, "")
	if both != "alpha\nbeta\ngamma\ndelta\n" {
		t.Fatalf("two files gave %q", both)
	}
	if _, _, status := runApplet(t, "less", []string{filepath.Join(dir, "absent")}, ""); status == 0 {
		t.Fatal("a missing file was accepted")
	}
	if help, _, _ := runApplet(t, "less", []string{"-h"}, ""); !strings.Contains(help, "forward one screen") {
		t.Fatalf("-h gave %q", help)
	}
}

// newTestPager builds a pager over numbered lines and a screen of the given size.
func newTestPager(t *testing.T, lines []string, width, height int) (*lessPager, tcell.SimulationScreen) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(width, height)
	pager := &lessPager{lines: lines, name: "f.txt", screen: screen, width: width, height: height}
	return pager, screen
}

func rowOf(t *testing.T, screen tcell.SimulationScreen, row, width int) string {
	t.Helper()
	var out strings.Builder
	for column := 0; column < width; column++ {
		text, _, _ := screen.Get(column, row)
		if text == "" {
			text = " "
		}
		out.WriteString(text)
	}
	return strings.TrimRight(out.String(), " ")
}

func lessNumberedLines(count int) []string {
	lines := make([]string, 0, count)
	for index := 1; index <= count; index++ {
		lines = append(lines, "line"+strconv.Itoa(index))
	}
	return lines
}

func TestLessScrolling(t *testing.T) {
	t.Parallel()
	// Twenty lines and a screen five rows tall: four of text and one for the prompt.
	pager, screen := newTestPager(t, lessNumberedLines(20), 40, 5)
	pager.draw()
	if got := rowOf(t, screen, 0, 40); got != "line1" {
		t.Fatalf("the first row is %q", got)
	}
	if got := rowOf(t, screen, 3, 40); got != "line4" {
		t.Fatalf("the fourth row is %q", got)
	}

	press := func(key rune) {
		pager.handle(tcell.NewEventKey(tcell.KeyRune, key, tcell.ModNone))
		pager.draw()
	}
	press(' ')
	if got := rowOf(t, screen, 0, 40); got != "line5" {
		t.Fatalf("after a page forward the first row is %q", got)
	}
	press('b')
	if got := rowOf(t, screen, 0, 40); got != "line1" {
		t.Fatalf("after a page back the first row is %q", got)
	}
	press('j')
	if got := rowOf(t, screen, 0, 40); got != "line2" {
		t.Fatalf("after a line forward the first row is %q", got)
	}
	press('k')
	if got := rowOf(t, screen, 0, 40); got != "line1" {
		t.Fatalf("after a line back the first row is %q", got)
	}

	// G goes to the end, and the last screenful is the last screenful: the final line
	// sits on the bottom text row rather than scrolling off into blank space.
	press('G')
	if got := rowOf(t, screen, 3, 40); got != "line20" {
		t.Fatalf("after G the last text row is %q", got)
	}
	if got := rowOf(t, screen, 0, 40); got != "line17" {
		t.Fatalf("after G the first row is %q", got)
	}
	// Paging forward at the end does nothing, rather than scrolling past the text.
	press(' ')
	if got := rowOf(t, screen, 0, 40); got != "line17" {
		t.Fatalf("paging past the end moved to %q", got)
	}
	press('g')
	if got := rowOf(t, screen, 0, 40); got != "line1" {
		t.Fatalf("after g the first row is %q", got)
	}
	press('q')
	if !pager.quit {
		t.Fatal("q did not quit")
	}
}

// TestLessShortFile covers the screen that is taller than the text, where the tildes say
// the file has ended rather than the screen having run out.
func TestLessShortFile(t *testing.T) {
	t.Parallel()
	pager, screen := newTestPager(t, []string{"only"}, 20, 5)
	pager.draw()
	if got := rowOf(t, screen, 0, 20); got != "only" {
		t.Fatalf("the first row is %q", got)
	}
	for row := 1; row < 4; row++ {
		if got := rowOf(t, screen, row, 20); got != "~" {
			t.Fatalf("row %d past the end is %q, want a tilde", row, got)
		}
	}
	if got := rowOf(t, screen, 4, 20); !strings.Contains(got, "(END)") {
		t.Fatalf("the prompt is %q", got)
	}
}

func TestLessLineNumbersAndChopping(t *testing.T) {
	t.Parallel()
	pager, screen := newTestPager(t, []string{"alpha", strings.Repeat("x", 40)}, 20, 4)
	pager.settings.numbers = true
	pager.draw()
	if got := rowOf(t, screen, 0, 20); got != "     1  alpha" {
		t.Fatalf("the numbered row is %q", got)
	}
	// Without -S a long line is left as it is and the terminal folds it; with -S it is
	// cut, so one row is one line.
	pager.settings.numbers = false
	pager.settings.chop = true
	pager.draw()
	if got := rowOf(t, screen, 1, 20); len(got) != 20 {
		t.Fatalf("the chopped row is %d wide: %q", len(got), got)
	}
}

func TestLessSearch(t *testing.T) {
	t.Parallel()
	lines := []string{"alpha", "beta", "gamma", "Beta"}
	for _, testcase := range []struct {
		name       string
		pattern    string
		from       int
		forward    bool
		ignoreCase bool
		want       int
	}{
		{name: "forward", pattern: "gamma", from: 0, forward: true, want: 2},
		{name: "backward", pattern: "alpha", from: 3, want: 0},
		{name: "case matters by default", pattern: "beta", from: 2, forward: true, want: -1},
		{name: "ignoring case finds it", pattern: "beta", from: 2, forward: true, ignoreCase: true, want: 3},
		{
			// It does not wrap: a pager that quietly took the reader back to the top
			// would make "no more" indistinguishable from "none at all".
			name: "no wrap", pattern: "alpha", from: 1, forward: true, want: -1,
		},
		{name: "a pattern that is not there", pattern: "zzz", from: 0, forward: true, want: -1},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got := lessSearchFrom(lines, testcase.pattern, testcase.from, testcase.forward, testcase.ignoreCase)
			if got != testcase.want {
				t.Fatalf("search for %q from %d = %d, want %d", testcase.pattern, testcase.from, got, testcase.want)
			}
		})
	}

	// Typing a search interactively: `/`, the pattern, then Enter.
	pager, _ := newTestPager(t, lines, 20, 3)
	pager.handle(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	for _, character := range "gamma" {
		pager.handle(tcell.NewEventKey(tcell.KeyRune, character, tcell.ModNone))
	}
	if pager.prompt() != "/gamma" {
		t.Fatalf("while typing, the prompt is %q", pager.prompt())
	}
	pager.handle(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if pager.top != 2 {
		t.Fatalf("the search landed on %d, want 2", pager.top)
	}
	// Escape abandons it and leaves the view where it was.
	pager.handle(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	pager.handle(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if pager.searching != 0 || pager.top != 2 {
		t.Fatalf("escape left searching=%d top=%d", pager.searching, pager.top)
	}
}
