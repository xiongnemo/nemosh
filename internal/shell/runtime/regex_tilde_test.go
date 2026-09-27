package runtime_test

import "testing"

// A tilde begins the right side of `=~` as it begins any other word, and expands there, as
// in both references: with HOME=foo, `[[ foo =~ ~ ]]` is true. It was left as a `~`. The
// directory it gives is matched as the characters it is, as bash matches it and the Oils
// expectations have it: with HOME='^a$', `[[ $HOME =~ ~ ]]` is true.
func TestRuntime_regexOperandExpandsATilde(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"HOME=foo\n[[ ~ =~ $HOME ]]; echo $?\n[[ $HOME =~ ~ ]]; echo $?\n[[ foo/x =~ ~/x ]]; echo $?\n", "0\n0\n0\n"},
		{"HOME='^a$'\n[[ ~ =~ $HOME ]]; echo $?\n[[ $HOME =~ ~ ]]; echo $?\n", "1\n0\n"},
		{"HOME=foo\n[[ '~' =~ '~' ]]; echo $?\n[[ foo =~ \\~ ]]; echo $?\n", "0\n1\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
