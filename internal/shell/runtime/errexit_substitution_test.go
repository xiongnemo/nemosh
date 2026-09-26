package runtime_test

import "testing"

// A command substitution does not inherit `set -e`, in either reference: inside $( ) a
// failing command does not end it, and $- has no e. Here it did, so under the `set -e` that
// heads a great many scripts, `x=$(false; echo hi)` ended the whole script where both give
// x the hi. The substitution's own status still counts, so `x=$(false)` still ends it, and
// a `set -e` inside the substitution still ends that.
func TestRuntime_commandSubstitutionDoesNotInheritErrexit(t *testing.T) {
	tests := []struct {
		script, want string
		status       int
	}{
		{`set -e; x=$(false; echo hi); echo "[$x]"`, "[hi]\n", 0},
		{`set -e; echo "$(false; echo inner)"; echo after`, "inner\nafter\n", 0},
		{`set -e; f() { false; echo in-f; }; x=$(f); echo "[$x]"`, "[in-f]\n", 0},
		{`set -e; x=$(echo "$-"); case $x in *e*) echo inherited;; *) echo not;; esac`, "not\n", 0},
		{`set -e; x=$(false); echo not-reached`, "", 1},
		{`set -e; x=$(set -e; false; echo hi); echo "[$x]"`, "", 1},
		// bash's `shopt -s inherit_errexit` asks for `set -e` inside; busybox has no shopt.
		{`set -e; shopt -s inherit_errexit; x=$(false; echo hi); echo "[$x]"`, "", 1},
		{`set -e; shopt -s inherit_errexit; x=$(echo "$-"); case $x in *e*) echo inherited;; *) echo not;; esac`, "inherited\n", 0},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != test.status {
				t.Errorf("got %q/%d, want %q/%d", stdout, status, test.want, test.status)
			}
		})
	}
}
