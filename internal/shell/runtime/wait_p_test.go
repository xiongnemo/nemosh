package runtime_test

import "testing"

// `wait -p NAME` puts the id of the job the status is for in NAME, as `$!` names it, and
// unsets NAME first, so a wait that answers for no one job leaves it unset. The answers are
// bash 5.3's; busybox's wait has no -p.
func TestRuntime_waitPNamesTheJob(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`(exit 4) & p=$!; wait -n -p v; echo "st=$? $([ "$v" = "$p" ] && echo same)"`, "st=4 same\n"},
		{`sleep 0.1 & p=$!; wait -p w "$p"; echo "st=$? $([ "$w" = "$p" ] && echo same)"`, "st=0 same\n"},
		{`(exit 3) & p=$!; wait -p v -n; echo "st=$? $([ "$v" = "$p" ] && echo same)"`, "st=3 same\n"},
		{`v=old; wait -p v; echo "st=$? [${v-unset}]"; v=old; wait -n -p v; echo "st=$? [${v-unset}]"`, "st=0 [unset]\nst=127 [unset]\n"},
		{`wait -p 2>/dev/null; echo "st=$?"; wait -p 'a b' 2>/dev/null; echo "st=$?"; wait -x 2>/dev/null; echo "st=$?"`, "st=2\nst=1\nst=2\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
