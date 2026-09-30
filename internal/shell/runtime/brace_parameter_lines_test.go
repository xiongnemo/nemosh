package runtime_test

import "testing"

// An unquoted ${...} may go on past its line, as busybox and bash read it: the newline is the
// expansion's. The line ended at it, and the script was refused as missing a }. What is inside
// is no comment and no group, and a quote in it opens as it does outside. busybox's ash_test
// param_expand_alt2.
func TestRuntime_aBraceParameterMaySpanLines(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"echo Unquoted: H${$+\n}H\n", "Unquoted: H H\n"},
		{"echo \"Quoted: H${$+\n}H\"\n", "Quoted: H\nH\n"},
		{"x=1; echo ${x:+a\nb}\n", "a b\n"},
		{"echo ${u:-${v:-\nin}}\n", "in\n"},
		{"echo ${u:-a # not a comment\n}\n", "a # not a comment\n"},
		{"echo ${u:-'a\nb'}\n", "a\nb\n"},
		{"f() { echo ${u:-\nfn}; }; f\n", "fn\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
	if stdout, status := runScriptCapturing("echo ${x\n"); stdout != "" || status == 0 {
		t.Errorf("got %q/%d, want an unclosed ${ refused, as in both references", stdout, status)
	}
}
