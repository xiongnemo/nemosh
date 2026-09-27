package runtime_test

import (
	"os"
	"strings"
	"testing"
)

// A new shell has OPTIND=1, PS4="+ " and HOSTNAME, as busybox's has them (shell/ash.c's
// init): none of them was set, so `echo $OPTIND` before a getopts loop was empty and `set -x`
// had no PS4 to show. None is exported by being set.
func TestRuntime_startupVariables(t *testing.T) {
	hostname, err := os.Hostname()
	if err != nil {
		t.Skip("no host name to compare with")
	}

	// When
	status, stdout, stderr := runSetScript(t,
		"echo \"[$OPTIND] [$PS4] [$HOSTNAME]\"\nenv | grep -E '^(OPTIND|PS4)=' || echo none-exported\n")

	// Then
	if want := "[1] [+ ] [" + hostname + "]\nnone-exported\n"; !strings.EqualFold(stdout, want) || status != 0 {
		t.Fatalf("got %q/%d, want %q/0; stderr = %q", stdout, status, want, stderr)
	}
}
