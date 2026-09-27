package runtime_test

import "testing"

// `trap -P condition...` prints each condition's action alone, bash 5.3's form for capturing
// one: an empty line for an ignored condition, none for one with no trap, and a condition is
// required. It armed the condition with a command named -P. busybox has no -P.
func TestRuntime_trapPPrintsTheActions(t *testing.T) {
	tests := []struct {
		script, want string
		status       int
	}{
		{`trap 'echo hi' INT; trap '' TERM; trap -P INT TERM HUP; echo "st=$?"`, "echo hi\n\nst=0\n", 0},
		{`trap 'echo hi' INT; trap -P INT >/dev/null; trap`, "trap -- 'echo hi' INT\n", 0},
		{`x=$(trap 'echo a; echo b' INT; trap -P INT); echo "[$x]"`, "[echo a; echo b]\n", 0},
		{`trap -P 2>/dev/null; echo "st=$?"; trap -P NOSUCH 2>/dev/null; echo "st=$?"`, "st=2\nst=1\n", 0},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != test.status {
				t.Errorf("got %q/%d, want %q/%d, as bash answers", stdout, status, test.want, test.status)
			}
		})
	}
}
