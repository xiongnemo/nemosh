package applets_test

import (
	"strings"
	"testing"
)

// The whole point of the translation: which characters need a backslash runs in both
// directions between BRE and Go's syntax. Each row was measured against busybox-w32.
func TestSed_translatesBasicRegularExpressions(t *testing.T) {
	tests := []struct {
		name   string
		script string
		input  string
		want   string
	}{
		{name: "any character", script: "s/x.y/A/", input: "x-y\n", want: "A\n"},
		{name: "an escaped dot is a dot", script: `s/x\.y/D/`, input: "x-y\n", want: "x-y\n"},
		{name: "greedy star", script: "s|.*world|W|", input: "hello world\n", want: "W\n"},
		{name: "a group and a backreference", script: `s/\(a\+\)/[\1]/`, input: "baaad\n", want: "b[aaa]d\n"},
		{name: "two groups, swapped", script: `s/\(.\)\(.\)/\2\1/`, input: "ab\n", want: "ba\n"},
		{name: "an interval", script: `s/o\{2\}/O/`, input: "foo\n", want: "fO\n"},
		{name: "a bare pipe is literal", script: "s/a|b/P/", input: "a|b\n", want: "P\n"},
		{name: "an escaped pipe alternates", script: `s/\(a\|b\)/(&)/g`, input: "ab\n", want: "(a)(b)\n"},
		{name: "bare parentheses are literal", script: "s/(x)/P/", input: "(x)\n", want: "P\n"},
		{name: "a bare plus is literal", script: "s/a+/P/", input: "a+\n", want: "P\n"},
		{name: "a character class", script: "s/[[:digit:]]/#/g", input: "a1b2\n", want: "a#b#\n"},
		{name: "a negated bracket", script: "s/[^a-z]//g", input: "a1b2c\n", want: "abc\n"},
		{name: "a bracket holding a bracket", script: "s/[]]/B/", input: "a]b\n", want: "aBb\n"},
		{name: "an anchor at the start", script: "s/^a/A/", input: "aa\n", want: "Aa\n"},
		{name: "an anchor at the end", script: "s/a$/A/", input: "aa\n", want: "aA\n"},
		{name: "a caret in the middle is literal", script: "s/a^b/C/", input: "a^b\n", want: "C\n"},
		{name: "a star with nothing to repeat", script: "s/*/S/", input: "*x\n", want: "Sx\n"},
		{name: "the whole match", script: "s/foo/[&]/", input: "foo\n", want: "[foo]\n"},
		{name: "an escaped ampersand", script: `s/foo/\&/`, input: "foo\n", want: "&\n"},
		{name: "a dollar in the replacement", script: "s/a/$/", input: "a\n", want: "$\n"},
		// An empty match is a match, which is what leaves the digits alone here.
		{name: "an empty match", script: "s/[0-9]*//", input: "foo123\n", want: "foo123\n"},
		{name: "the second occurrence only", script: "s/a/X/2", input: "aaa\n", want: "aXa\n"},
		{name: "the second onward", script: "s/a/X/2g", input: "aaa\n", want: "aXX\n"},
		// An escaped delimiter is the delimiter, not the end of the pattern. This used to
		// fail with `sed: unknown option to 's': /`.
		{name: "an escaped delimiter", script: `s/a\/b/S/`, input: "a/b\n", want: "S\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			stdout, stderr, err := runSed(t, test.input, test.script)

			// Then
			if err != nil {
				t.Fatalf("sed %q: %v (stderr %q)", test.script, err, stderr)
			}
			if stdout != test.want {
				t.Fatalf("sed %q on %q = %q, want %q", test.script, test.input, stdout, test.want)
			}
		})
	}
}

// RE2 has no backreferences, so a pattern that uses one cannot be honoured. Refusing by name
// beats matching `\1` as a literal digit, which is what the old literal search did.
func TestSed_refusesABackreferenceInThePattern(t *testing.T) {
	// When
	_, stderr, err := runSed(t, "aa\n", `s/\(a\)\1/x/`)

	// Then
	if err == nil {
		t.Fatal("expected a backreference in the pattern to be refused")
	}
	if message := err.Error() + stderr; !strings.Contains(message, "RE2") {
		t.Fatalf("diagnostic = %q, want it to say why", message)
	}
}
