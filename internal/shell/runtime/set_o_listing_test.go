package runtime_test

import (
	"strings"
	"testing"
)

// `set -o` prints busybox's table, each name padded to sixteen with blanks and no tab, and a
// division by zero is busybox's `divide by zero`. The table had a tab after the name, and the
// error said "division by zero".
func TestRuntime_setOAndDivisionAreWordedAsBusyboxWordsThem(t *testing.T) {
	stdout, _ := runScriptCapturing("set -e; set -o | grep -E '^(errexit|noglob) '\n")
	if want := "errexit         on\nnoglob          off\n"; stdout != want {
		t.Errorf("set -o: got %q, want %q, as busybox prints it", stdout, want)
	}
	status, _, stderr := runSetScript(t, "echo $((5 % 0))\n")
	if status != 2 || !strings.Contains(stderr, "divide by zero") {
		t.Errorf("$((5 %% 0)): got %d, %q; want 2 and busybox's divide by zero", status, stderr)
	}
}
