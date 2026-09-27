package runtime_test

import "testing"

// A blank a backslash escapes at the end of a line is part of the last word, as in
// busybox-w32 and bash: `echo a\ ` prints `a ` and a space. The line's blanks were trimmed
// before it was read, the escaped one with them, which left the backslash last on the line --
// "trailing backslash", and the whole script refused. A substitution's body, `$(...)` or
// backquoted, is read the same way.
func TestRuntime_lineEndingInAnEscapedBlankKeepsIt(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"echo a\\ \necho next\n", "a \nnext\n"},
		{"echo \"[$(echo b\\ )]\" \"[`echo c\\\\ `]\"\n", "[b ] [c ]\n"},
		{"echo [1 `echo \\ `]\necho \"[1 `echo \\ `]\"\n", "[1 ]\n[1  ]\n"},
		{"echo a\\\\ \n", "a\\\n"},
		{"echo a\\\t\n", "a\t\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox-w32 and bash answer", stdout, status, test.want)
			}
		})
	}
}
