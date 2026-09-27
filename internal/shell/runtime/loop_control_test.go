package runtime_test

import (
	"strings"
	"testing"
)

// A break or continue in a loop's condition is that loop's, as in both references: `while
// break; do ...; done` ends it. It went up to the loop outside, or to the top of the script,
// so a nested one ended the outer loop and a lone one ended the script.
func TestRuntime_breakInALoopConditionIsThatLoops(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"while break; do echo x; done; echo done", "done\n"},
		{"for i in 1 2 3; do echo i=$i; while break; do echo x; done; done; echo done", "i=1\ni=2\ni=3\ndone\n"},
		{"n=0; until [ $n -ge 2 ] && break; do n=$((n+1)); done; echo n=$n", "n=2\n"},
		{"for i in 1 2; do while continue 2; do echo x; done; echo never; done; echo end", "end\n"},
		// Through `command`, and bash's `builtin`, the builtin still transfers control.
		{"for i in 1 2 3; do [ $i = 2 ] && command continue; echo $i; done", "1\n3\n"},
		{"for i in 1 2 3; do echo $i; builtin break; done", "1\n"},
		{"f() { command return 7; echo no; }; f; echo st=$?", "st=7\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as both references answer", stdout, status, test.want)
			}
		})
	}
}

// A count break or continue cannot read is a special builtin's error, and it ends the script
// with status 2, as busybox's does -- a word, 0 or a negative number. It set status 1 and the
// loop went on, so `while true; do ...; break $n; done` with n misspelled never ended.
func TestRuntime_aBadLoopCountEndsTheScript(t *testing.T) {
	for _, count := range []string{"oops", "0", "-1"} {
		t.Run(count, func(t *testing.T) {
			stdout, status := runScriptCapturing("while true; do echo hi; break " + count + "; done; echo after\n")
			if stdout != "hi\n" || status != 2 || strings.Contains(stdout, "after") {
				t.Errorf("got %q/%d, want %q/2, as busybox answers", stdout, status, "hi\n")
			}
		})
	}
}
