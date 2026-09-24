package runtime

import "testing"

// POSIX character classes in bracket expressions. The class's own `]` closed the bracket, so
// every one matched nothing; both references match them.
func TestPattern_matchesCharacterClasses(t *testing.T) {
	for _, test := range []struct {
		pattern, value string
		want           bool
	}{
		{"[[:upper:]]", "A", true},
		{"[[:upper:]]", "a", false},
		{"[[:lower:]]*", "abc", true},
		{"[[:digit:]][[:digit:]]", "42", true},
		{"[![:digit:]]", "x", true},
		{"[![:digit:]]", "7", false},
		{"[[:alnum:]_]", "_", true},
		{"[[:space:]]", "\t", true},
		{"[[:blank:]]", "\n", false},
		{"[[:punct:]]", "!", true},
		{"[[:xdigit:]]", "F", true},
		{"[[:alpha:][:digit:]]", "9", true},
		{"[[:bogus:]]", "a", false},
		{"[]]", "]", true},
		{"[a-c]", "b", true},
	} {
		if got := matchShellPattern(test.pattern, test.value); got != test.want {
			t.Errorf("matchShellPattern(%q, %q) = %v, want %v", test.pattern, test.value, got, test.want)
		}
	}
}
