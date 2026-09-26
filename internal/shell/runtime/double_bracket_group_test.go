package runtime_test

import "testing"

// Parentheses inside `[[ ]]` group its expression. They were taken for a subshell and left a
// placeholder word behind, and a lone non-empty word is true, so every parenthesised test was
// true whatever it said -- `[[ ( 1 -eq 2 ) ]]` included -- and `!` before one made it false.
// The parentheses need no blanks around them, and the operand of `=~` and an extended pattern
// keep theirs. The answers are bash 5.3's; busybox-w32 has no `[[`.
func TestRuntime_doubleBracketParenthesesGroup(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"[[ ( 1 -eq 2 ) ]]; echo $?", "1\n"},
		{"[[ (a == a) ]]; echo $?", "0\n"},
		{"[[ ! ( a == b ) ]]; echo $?", "0\n"},
		{"[[ ( a == b ) || ( c == c ) ]]; echo $?", "0\n"},
		{"[[ (a == b) || (c == d) ]]; echo $?", "1\n"},
		{"x=5; [[ $x -gt 3 && ( $x -lt 4 || $x -eq 99 ) ]]; echo $?", "1\n"},
		{"if [[ ( -n x ) && ( 2 -gt 1 ) ]]; then echo yes; fi", "yes\n"},
		{"f() [[ ( 1 -eq 2 ) ]]\nf; echo $?", "1\n"},
		{"[[ ( $(echo a) == a ) ]]; echo $?", "0\n"},
		{"[[ (1 -eq 1)]]; echo $?", "0\n"},
		{"[[ '(' == '(' ]]; echo $?", "0\n"},
		{"[[ x =~ (x|y) ]]; echo $? ${BASH_REMATCH[1]}", "0 x\n"},
		{"[[ ab == @(ab|cd) ]]; echo $?", "0\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
