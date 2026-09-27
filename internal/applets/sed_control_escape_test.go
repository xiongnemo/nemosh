package applets_test

import "testing"

// busybox's sed turns `\n`, `\t` and `\r` into the characters they name anywhere in a
// pattern, in a bracket expression too, before the pattern is compiled; any other backslash
// in a bracket is itself, and so is the first of a `\\`. In a bracket they were a backslash
// and a letter here, so `s/[ \t]\+/ /g` ate the t off the end of a word and left the tab.
// Each row measured against busybox-w32.
func TestSed_readsControlEscapesInABracket(t *testing.T) {
	tests := []struct {
		name, script, input, want string
	}{
		{"a tab", `s/[ \t]\+/_/g`, "nounset\t\toff x\n", "nounset_off_x\n"},
		{"a newline", `N;s/[\n]/+/`, "a\nb\n", "a+b\n"},
		{"a carriage return", `s/[\r]//`, "a\rb\n", "ab\n"},
		{"not a backslash and an n", `s/[\n]/X/g`, `a\nb` + "\n", `a\nb` + "\n"},
		{"an escaped backslash before n", `s/[\\n]/X/g`, `a\nb` + "\n", "aXXb\n"},
		{"any other backslash", `s/[\*]/X/g`, `a\b*c` + "\n", "aXbXc\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := runSed(t, test.input, test.script)
			if err != nil {
				t.Fatalf("sed %q: %v (stderr %q)", test.script, err, stderr)
			}
			if stdout != test.want {
				t.Fatalf("sed %q on %q = %q, want %q, as busybox answers", test.script, test.input, stdout, test.want)
			}
		})
	}
}
