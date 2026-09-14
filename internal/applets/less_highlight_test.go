package applets

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// **What a search found is drawn highlighted.**
//
// Asked for at a terminal: `/` moved to the match and left it looking like every other
// line, so there was nothing to tell you *where* on the line it had matched.
//
// busybox puts ESC[7m -- reverse video -- around every part of a line that matches, not
// just the first (miscutils/less.c:867-889). It also refuses to highlight an empty match,
// and says why in its own comment: "if even \"\" matches, treat it as not a match". Both
// are followed here, the second because a pattern like `x*` matches at every position and
// would otherwise paint the whole file.

// highlightMask draws one row as a picture of which cells are reversed, so a failure says
// where the highlight went rather than that it went somewhere.
func highlightMask(t *testing.T, screen tcell.SimulationScreen, row, width int) string {
	t.Helper()
	var mask strings.Builder
	for column := range width {
		_, style, _ := screen.Get(column, row)
		_, _, attributes := style.Decompose()
		if attributes&tcell.AttrReverse != 0 {
			mask.WriteByte('^')
			continue
		}
		mask.WriteByte('.')
	}
	return strings.TrimRight(mask.String(), ".")
}

func TestLessHighlightsWhatTheSearchFound(t *testing.T) {
	for _, testcase := range []struct {
		name    string
		lines   []string
		pattern string
		numbers bool
		want    string
	}{
		{
			name:    "the matched word, not the whole line",
			lines:   []string{"hello world"},
			pattern: "world",
			want:    "......^^^^^",
		},
		{
			name:    "every match on the line, as busybox does",
			lines:   []string{"ab cd ab"},
			pattern: "ab",
			want:    "^^....^^",
		},
		{
			name:    "an empty match highlights nothing",
			lines:   []string{"hello"},
			pattern: "x*",
			want:    "",
		},
		{
			name:    "a line with no match is left alone",
			lines:   []string{"nothing here"},
			pattern: "absent",
			want:    "",
		},
		{
			name:    "line numbers shift the highlight, not the answer",
			lines:   []string{"hello world"},
			pattern: "hello",
			numbers: true,
			want:    "........^^^^^",
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			pager, screen := newTestPager(t, testcase.lines, 40, 4)
			pager.pattern = testcase.pattern
			pager.settings.numbers = testcase.numbers
			pager.draw()
			if got := highlightMask(t, screen, 0, 40); got != testcase.want {
				t.Errorf("highlight was\n  %q\nwant\n  %q\n(row drew %q)",
					got, testcase.want, rowOf(t, screen, 0, 40))
			}
		})
	}
}

// TestLessHighlightIsCaseInsensitiveWithI keeps the highlight and the jump using one
// pattern: -I changes what counts as a match, and a highlight that disagreed with where
// the search landed would be worse than none.
func TestLessHighlightIsCaseInsensitiveWithI(t *testing.T) {
	pager, screen := newTestPager(t, []string{"Hello World"}, 40, 4)
	pager.settings.ignoreCase = true
	pager.pattern = "hello"
	pager.draw()
	if got := highlightMask(t, screen, 0, 40); got != "^^^^^" {
		t.Errorf("with -I the highlight was %q, want the first five cells", got)
	}
}

// TestLessHighlightsNothingWithoutASearch is the state a pager opens in.
func TestLessHighlightsNothingWithoutASearch(t *testing.T) {
	pager, screen := newTestPager(t, []string{"hello world"}, 40, 4)
	pager.draw()
	if got := highlightMask(t, screen, 0, 40); got != "" {
		t.Errorf("a pager with no pattern highlighted %q", got)
	}
}
