package applets

import (
	"strings"
	"testing"
)

// -path's pattern compiler, which was at 26.7%: only the plain `*` case was
// exercised, so the bracket expressions -- the fiddly part, and the reason it
// translates to a regexp at all -- were untested.
//
// The property that makes -path different from -name is at the top: its wildcards
// **cross separators**, because GNU find and busybox both call fnmatch without
// FNM_PATHNAME. Go's path.Match hard-codes the opposite, which is why -name can use
// it and -path cannot, so that difference is the first thing asserted.

func TestCompileFindPathPattern_wildcardsCrossSeparators(t *testing.T) {
	for _, test := range []struct {
		pattern string
		matches []string
		nomatch []string
	}{
		{
			// The measured case: `find . -path "./sub*"` answers ./sub and
			// ./sub/c.txt, because the star crosses the slash.
			pattern: "./sub*",
			matches: []string{"./sub", "./sub/c.txt", "./sub/deep/d.txt", "./subdir"},
			nomatch: []string{"./other", "./a/sub", "sub"},
		},
		{
			pattern: "*.txt",
			matches: []string{"a.txt", "./a.txt", "./deep/nested/a.txt"},
			nomatch: []string{"a.md", "a.txt.gz"},
		},
		{
			// A star in the middle spans as many separators as it needs to.
			pattern: "./a/*/d.txt",
			matches: []string{"./a/b/d.txt", "./a/b/c/d.txt"},
			nomatch: []string{"./a/d.txt", "./b/c/d.txt"},
		},
		{
			// `?` is exactly one character, and it too may be a separator.
			pattern: "a?c",
			matches: []string{"abc", "a/c", "a.c"},
			nomatch: []string{"ac", "abbc"},
		},
	} {
		t.Run(test.pattern, func(t *testing.T) {
			expression, err := compileFindPathPattern(test.pattern)
			if err != nil {
				t.Fatalf("compileFindPathPattern(%q): %v", test.pattern, err)
			}
			for _, path := range test.matches {
				if !expression.MatchString(path) {
					t.Errorf("%q should match %q", test.pattern, path)
				}
			}
			for _, path := range test.nomatch {
				if expression.MatchString(path) {
					t.Errorf("%q should not match %q", test.pattern, path)
				}
			}
		})
	}
}

// The bracket expressions, which is where the untested code was.
func TestCompileFindPathPattern_bracketExpressions(t *testing.T) {
	for _, test := range []struct {
		pattern string
		matches []string
		nomatch []string
	}{
		{pattern: "[abc].txt", matches: []string{"a.txt", "b.txt", "c.txt"}, nomatch: []string{"d.txt", "ab.txt"}},
		{pattern: "[a-c].txt", matches: []string{"a.txt", "b.txt", "c.txt"}, nomatch: []string{"d.txt"}},
		// A `!` negation is the shell's spelling and becomes regexp's `^`.
		{pattern: "[!abc].txt", matches: []string{"d.txt", "z.txt"}, nomatch: []string{"a.txt", "c.txt"}},
		// `^` is accepted as a negation too, which some shells allow.
		{pattern: "[^abc].txt", matches: []string{"d.txt"}, nomatch: []string{"a.txt"}},
		// A `]` in the first position is a literal, which is the shell's rule.
		{pattern: "[]a].txt", matches: []string{"].txt", "a.txt"}, nomatch: []string{"b.txt"}},
		// And after a negation.
		{pattern: "[!]a].txt", matches: []string{"b.txt"}, nomatch: []string{"].txt", "a.txt"}},
		// A caret that is not first is a literal, and must be escaped on the way
		// into the regexp or it would negate.
		{pattern: "[a^].txt", matches: []string{"a.txt", "^.txt"}, nomatch: []string{"b.txt"}},
		// A backslash inside a class is a literal backslash, escaped for regexp.
		{pattern: `[a\].txt`, matches: []string{"a.txt", `\.txt`}, nomatch: []string{"b.txt"}},
		// Two classes in one pattern.
		{pattern: "[ab][12]", matches: []string{"a1", "b2"}, nomatch: []string{"a3", "c1"}},
		// A class combined with a star.
		{pattern: "[ab]*.txt", matches: []string{"a.txt", "b/deep/x.txt"}, nomatch: []string{"c.txt"}},
	} {
		t.Run(test.pattern, func(t *testing.T) {
			expression, err := compileFindPathPattern(test.pattern)
			if err != nil {
				t.Fatalf("compileFindPathPattern(%q): %v", test.pattern, err)
			}
			for _, path := range test.matches {
				if !expression.MatchString(path) {
					t.Errorf("%q should match %q", test.pattern, path)
				}
			}
			for _, path := range test.nomatch {
				if expression.MatchString(path) {
					t.Errorf("%q should not match %q", test.pattern, path)
				}
			}
		})
	}
}

// Regexp metacharacters in a pattern are literals, because a glob is not a regexp.
// Without quoting, `-path "a.b"` would match "axb" and `-path "a+"` would be a
// syntax error rather than a file name.
func TestCompileFindPathPattern_treatsRegexpMetacharactersAsLiterals(t *testing.T) {
	for _, test := range []struct {
		pattern string
		matches string
		nomatch string
	}{
		{pattern: "a.b", matches: "a.b", nomatch: "axb"},
		{pattern: "a+b", matches: "a+b", nomatch: "aab"},
		{pattern: "a(b)c", matches: "a(b)c", nomatch: "abc"},
		{pattern: "a|b", matches: "a|b", nomatch: "a"},
		{pattern: "a{2}", matches: "a{2}", nomatch: "aa"},
		{pattern: "a$b", matches: "a$b", nomatch: "ab"},
		{pattern: "^ab", matches: "^ab", nomatch: "ab"},
	} {
		t.Run(test.pattern, func(t *testing.T) {
			expression, err := compileFindPathPattern(test.pattern)
			if err != nil {
				t.Fatalf("compileFindPathPattern(%q): %v", test.pattern, err)
			}
			if !expression.MatchString(test.matches) {
				t.Errorf("%q should match itself", test.pattern)
			}
			if expression.MatchString(test.nomatch) {
				t.Errorf("%q was read as a regexp: it matched %q", test.pattern, test.nomatch)
			}
		})
	}
}

// The pattern is anchored at both ends, which is what makes -path a whole-path
// test rather than a substring search.
func TestCompileFindPathPattern_isAnchoredAtBothEnds(t *testing.T) {
	expression, err := compileFindPathPattern("sub")
	if err != nil {
		t.Fatal(err)
	}
	if !expression.MatchString("sub") {
		t.Fatal("the exact path does not match")
	}
	for _, path := range []string{"subdir", "./sub", "asub", "a/sub/b"} {
		if expression.MatchString(path) {
			t.Errorf("the pattern matched %q, so it is not anchored", path)
		}
	}
}

// An unterminated bracket is refused at compile time rather than becoming a
// pattern that silently matches nothing -- which is the reason to compile before
// the walk starts.
func TestCompileFindPathPattern_refusesAnUnterminatedBracket(t *testing.T) {
	for _, pattern := range []string{"[abc", "a[bc", "[", "[!abc", "[]"} {
		_, err := compileFindPathPattern(pattern)
		if err == nil {
			t.Errorf("compileFindPathPattern(%q) was accepted", pattern)
			continue
		}
		if !strings.Contains(err.Error(), "[") {
			t.Errorf("the error for %q does not name the bracket: %v", pattern, err)
		}
	}
}

// A newline in a path still matches a star, which `(?s:.*)` is for: without the s
// flag, regexp's dot stops at a newline and a file whose name holds one would be
// invisible to -path.
func TestCompileFindPathPattern_starCrossesNewlines(t *testing.T) {
	expression, err := compileFindPathPattern("a*b")
	if err != nil {
		t.Fatal(err)
	}
	if !expression.MatchString("a\nb") {
		t.Fatal("a star does not cross a newline, so a path holding one is invisible")
	}
	single, err := compileFindPathPattern("a?b")
	if err != nil {
		t.Fatal(err)
	}
	if !single.MatchString("a\nb") {
		t.Fatal("a ? does not match a newline")
	}
}

// The empty pattern matches only the empty path, which is the anchoring being
// consistent rather than a special case.
func TestCompileFindPathPattern_emptyPattern(t *testing.T) {
	expression, err := compileFindPathPattern("")
	if err != nil {
		t.Fatal(err)
	}
	if !expression.MatchString("") {
		t.Fatal("the empty pattern does not match the empty path")
	}
	if expression.MatchString("a") {
		t.Fatal("the empty pattern matched something")
	}
}
