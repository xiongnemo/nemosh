package runtime_test

import "testing"

// `[[ -o name ]]`, `test -o name` and `[ -o name ]` ask whether a `set -o` option is on, and
// a name that is not one is off. Each was "unexpected" or "unknown operand", status 2. The
// answers are bash 5.3's; busybox-w32 has no option test. Between two expressions `-o` is
// still test's OR.
func TestRuntime_testOAsksWhetherAnOptionIsOn(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`[[ -o errexit ]]; echo $?; set -e; [[ -o errexit ]]; echo $?`, "1\n0\n"},
		{`[[ -o nosuch ]]; echo $?; x=pipefail; set -o pipefail; [[ -o $x && ! -o noglob ]]; echo $?`, "1\n0\n"},
		{`[[ -o braceexpand && -o interactive-comments && ! -o emacs ]]; echo $?`, "0\n"},
		{`test -o nounset; echo $?; set -o nounset; test -o nounset; echo $?; test -o _bad_; echo $?`, "1\n0\n1\n"},
		{`set -u; [ ! -o pipefail ]; echo $?; test -o nounset -a -o errexit; echo $?`, "0\n1\n"},
		{`[ -o ]; echo $?; [ a -o b ]; echo $?; [ "" -o "" ]; echo $?`, "0\n0\n1\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
