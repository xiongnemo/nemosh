package runtime_test

import "testing"

// Where `set -e` is ignored -- a condition, the left of `||` -- it is ignored inside a
// subshell there as well, even one that turns -e on itself. Each answer is busybox-w32's,
// measured; bash agrees. The subshell started with the exemption cleared and stopped at
// the first failure.
func TestErrexit_isIgnoredInASubshellWhereTheSubshellIsTested(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{script: "(set -e; false; echo not-here) || echo subshell-failed\n", want: "not-here\n"},
		{script: "if (set -e; false; echo inner); then echo then; fi\n", want: "inner\nthen\n"},
		{script: "while (set -e; false; echo w; exit 1); do :; done; echo end\n", want: "w\nend\n"},
		{script: "set -e\n(false) || echo caught\necho after\n", want: "caught\nafter\n"},
		{script: "set -e\n(false; echo no)\necho unreached\n", want: ""},
	} {
		if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
			t.Errorf("%q = %q, want %q", test.script, stdout, test.want)
		}
	}
}

// pwd into a pipe whose reader has gone ends quietly, as every other writer does and as
// busybox's does; it reported `pwd: pipeline downstream closed`.
func TestPwd_isQuietWhenTheReaderHasGone(t *testing.T) {
	_, stdout, stderr := runSetScript(t, "{ echo 1; sleep 0.2; pwd; echo after >&2; } | head -1\n")
	if stdout != "1\n" || stderr != "after\n" {
		t.Fatalf("stdout %q stderr %q, want the line head took and nothing from pwd", stdout, stderr)
	}
}
