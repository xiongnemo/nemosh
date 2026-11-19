package applets

import (
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"
)

// grep -i of one plain ASCII pattern lowers the line's ASCII and searches it as bytes, where Go's
// regexp under (?i) cannot use its literal prefix and walked every line: `grep -ci error` of
// 92 MB took 1.74 s, where busybox takes 1.24. It answers what (?i) answers: a letter whose case
// folds outside ASCII -- k to the Kelvin sign, s to the long s -- sends a line with any non-ASCII
// in it to the regexp.
func TestGrep_ignoreCaseOfAPlainPatternAgreesWithTheRegexp(t *testing.T) {
	random := rand.New(rand.NewPCG(5, 6))
	pieces := []string{"error", "ERROR", "Error", "eRrOr", "e", "r", "o", " ", "x", "é", "É", "中", "K", "ſ", "k", "K", "s", "S", "\xff", "ks"}
	var lines [][]byte
	for range 2000 {
		var line strings.Builder
		for range random.IntN(12) {
			line.WriteString(pieces[random.IntN(len(pieces))])
		}
		lines = append(lines, []byte(line.String()))
	}
	for _, pattern := range []string{"error", "Error", "ks", "s", "k", "x e", "o"} {
		flags := grepFlags{ignoreCase: true, patterns: []string{pattern}}
		expr, err := flags.compile()
		if err != nil {
			t.Fatal(err)
		}
		matcher := newGrepMatcher(flags, expr)
		if matcher.folded == nil {
			t.Fatalf("-i %q has no folded search", pattern)
		}
		for _, line := range lines {
			if got, want := matcher.match(line), expr.Match(line); got != want {
				t.Fatalf("-i %q of %q: the folded search said %v, the regexp %v", pattern, line, got, want)
			}
		}
	}
	for _, flags := range []grepFlags{
		{ignoreCase: true, patterns: []string{"err.r"}},
		{ignoreCase: true, patterns: []string{"a", "b"}},
		{ignoreCase: true, lineMatch: true, patterns: []string{"error"}},
		{ignoreCase: true, patterns: []string{"é"}},
		{patterns: []string{"error"}},
	} {
		expr := regexp.MustCompile("x")
		if newGrepMatcher(flags, expr).folded != nil {
			t.Errorf("%+v took the folded search", flags)
		}
	}
	fixed := grepFlags{ignoreCase: true, fixedString: true, patterns: []string{"a.c"}}
	expr, _ := fixed.compile()
	if matcher := newGrepMatcher(fixed, expr); matcher.folded == nil || !matcher.match([]byte("xA.Cx")) || matcher.match([]byte("abc")) {
		t.Error("-iF of a.c does not search for the dot itself")
	}
}

// grep -w of one plain pattern searches for it and looks at the bytes either side, where the
// regexp -w makes, `(?:\A|\W)(word)(?:\z|\W)`, begins with an alternation Go cannot take a literal
// prefix from: `grep -cw err` of 88 MB took 2.99 s, where busybox's takes 0.94. It answers what that
// regexp answers, an ASCII word character being the only one that is no edge, under -i too.
func TestGrep_wordOfAPlainPatternAgreesWithTheRegexp(t *testing.T) {
	random := rand.New(rand.NewPCG(7, 8))
	pieces := []string{"err", "ERR", "Err", "errs", "_", "e", "r", "s", " ", "-", ".", "x", "9", "é", "中", "\xff", "K", "k", "-x"}
	var lines [][]byte
	for range 2000 {
		var line strings.Builder
		for range random.IntN(10) {
			line.WriteString(pieces[random.IntN(len(pieces))])
		}
		lines = append(lines, []byte(line.String()))
	}
	for _, flags := range []grepFlags{
		{wordMatch: true, patterns: []string{"err"}},
		{wordMatch: true, patterns: []string{"e"}},
		{wordMatch: true, patterns: []string{"-x"}},
		{wordMatch: true, patterns: []string{"r s"}},
		{wordMatch: true, patterns: []string{"中"}},
		{wordMatch: true, fixedString: true, patterns: []string{"e.r"}},
		{wordMatch: true, ignoreCase: true, patterns: []string{"err"}},
		{wordMatch: true, ignoreCase: true, patterns: []string{"k"}},
	} {
		expr, err := flags.compile()
		if err != nil {
			t.Fatal(err)
		}
		matcher := newGrepMatcher(flags, expr)
		for _, line := range lines {
			if got, want := matcher.match(line), expr.Match(line); got != want {
				t.Fatalf("%+v of %q: the search said %v, the regexp %v", flags, line, got, want)
			}
		}
	}
	// The search, and not the regexp, answers: here a regexp that matches nothing.
	never := regexp.MustCompile(`$.^`)
	for _, test := range []struct {
		flags grepFlags
		line  string
	}{
		{grepFlags{wordMatch: true, patterns: []string{"err"}}, "an err here"},
		{grepFlags{wordMatch: true, ignoreCase: true, patterns: []string{"err"}}, "an Err here"},
	} {
		if !newGrepMatcher(test.flags, never).match([]byte(test.line)) {
			t.Errorf("%+v of %q asked the regexp", test.flags, test.line)
		}
	}
}
