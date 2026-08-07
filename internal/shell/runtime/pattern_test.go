package runtime

import "testing"

func TestMatchShellPattern_followsThePosixPatternGrammar(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		{name: "literal equal", pattern: "abc", value: "abc", want: true},
		{name: "literal unequal", pattern: "abc", value: "abd", want: false},
		{name: "star alone matches empty", pattern: "*", value: "", want: true},
		{name: "star alone matches anything", pattern: "*", value: "a/b c", want: true},
		{name: "star crosses a slash", pattern: "/a/*", value: "/a/b/c", want: true},
		{name: "leading star", pattern: "*.txt", value: "notes.txt", want: true},
		{name: "leading star needs the suffix", pattern: "*.txt", value: "notes.md", want: false},
		{name: "star in the middle", pattern: "a*d", value: "abcd", want: true},
		{name: "two stars backtrack", pattern: "*b*d", value: "abcbxd", want: true},
		{name: "question matches one", pattern: "a?c", value: "abc", want: true},
		{name: "question needs one", pattern: "a?c", value: "ac", want: false},
		{name: "question matches one rune not one byte", pattern: "?", value: "中", want: true},
		{name: "bracket set", pattern: "[abc]x", value: "bx", want: true},
		{name: "bracket set excludes", pattern: "[abc]x", value: "dx", want: false},
		{name: "bracket range", pattern: "[a-f]", value: "d", want: true},
		{name: "bracket range excludes", pattern: "[a-f]", value: "g", want: false},
		{name: "bracket negation with bang", pattern: "[!abc]", value: "d", want: true},
		{name: "bracket negation rejects a member", pattern: "[!abc]", value: "b", want: false},
		{name: "bracket negation with caret", pattern: "[^0-9]", value: "x", want: true},
		{name: "closing bracket first is data", pattern: "[]a]", value: "]", want: true},
		{name: "unmatched bracket is literal", pattern: "[abc", value: "[abc", want: true},
		{name: "backslash quotes a star", pattern: `a\*b`, value: "a*b", want: true},
		{name: "backslash quoted star is not a wildcard", pattern: `a\*b`, value: "axxb", want: false},
		{name: "trailing star matches empty tail", pattern: "ab*", value: "ab", want: true},
		{name: "pattern longer than value", pattern: "abc", value: "ab", want: false},
		{name: "empty pattern matches empty value", pattern: "", value: "", want: true},
		{name: "empty pattern rejects a value", pattern: "", value: "a", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := matchShellPattern(test.pattern, test.value)

			// Then
			if got != test.want {
				t.Fatalf("matchShellPattern(%q, %q) = %v, want %v", test.pattern, test.value, got, test.want)
			}
		})
	}
}

func TestSplitCaseAlternatives_cutsOnlyTheUnquotedBars(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    []string
	}{
		{name: "single", pattern: "a", want: []string{"a"}},
		{name: "two", pattern: "a|b", want: []string{"a", "b"}},
		{name: "three", pattern: "a|b|c", want: []string{"a", "b", "c"}},
		{name: "quoted bar is data", pattern: `a"|"b`, want: []string{`a"|"b`}},
		{name: "escaped bar is data", pattern: `a\|b`, want: []string{`a\|b`}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := splitCaseAlternatives(test.pattern)

			// Then
			if len(got) != len(test.want) {
				t.Fatalf("splitCaseAlternatives(%q) = %q, want %q", test.pattern, got, test.want)
			}
			for index := range got {
				if got[index] != test.want[index] {
					t.Fatalf("splitCaseAlternatives(%q) = %q, want %q", test.pattern, got, test.want)
				}
			}
		})
	}
}
