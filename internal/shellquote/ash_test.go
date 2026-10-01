package shellquote

import "testing"

// Ash quotes as busybox's set, alias and trace were measured to: a run of quotes goes in double
// quotes between single-quoted parts, and nothing else is changed.
func TestAsh_writesEachRunOfQuotesInDoubleQuotes(t *testing.T) {
	for value, want := range map[string]string{
		"":       `''`,
		"plain":  `'plain'`,
		"it's":   `'it'"'"'s'`,
		"'":      `''"'"`,
		"''":     `''"''"`,
		"a''b":   `'a'"''"'b'`,
		"'a":     `''"'"'a'`,
		"a'":     `'a'"'"`,
		"a\nb $": "'a\nb $'",
	} {
		if got := Ash(value); got != want {
			t.Errorf("Ash(%q) = %s, want %s", value, got, want)
		}
	}
}
