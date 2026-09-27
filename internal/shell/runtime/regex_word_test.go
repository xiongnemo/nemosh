package runtime_test

import "testing"

// The word after `=~` is read as bash reads it: a blank inside its parentheses is part of the
// regular expression, and a `)` it did not open closes a group of the condition. `(a  b)` was
// two words, the first an unbalanced `(a`, and in `[[ (x =~ x) ]]` the expression was `x)`.
// The answers are bash 5.3's; busybox has no `=~`.
func TestRuntime_regexWordKeepsItsParentheses(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"[[ (foo =~ foo) ]] && echo y", "y\n"},
		{"[[ 'a  b' =~ (a  b) ]] && echo y; [[ 'a b' =~ (a  b) ]] || echo n", "y\nn\n"},
		{"[[ 'a b' =~ (a b|c) ]] && echo y; [[ '  c' =~ (a|  c) ]] && echo z", "y\nz\n"},
		{"[[ ! (ab =~ x || ab =~ a) ]]; echo st=$?; [[ (ab =~ x || ab =~ a) ]]; echo st=$?", "st=1\nst=0\n"},
		{"[[ x =~ ( x ) ]]; echo st=$?", "st=1\n"},
		{"[[ ab =~ (a)(b) && 1 == 1 ]] && echo \"${BASH_REMATCH[2]}\"", "b\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
