package runtime_test

import "testing"

// A command substitution that runs nothing exits 0, as in busybox and bash, so a command made
// of one, or an assignment from one, leaves 0: `false; a=$()`. It left the status before it,
// which the empty script began with and never changed. busybox's ash_test emptytick and
// falsetick.
func TestRuntime_emptySubstitutionExitsZero(t *testing.T) {
	const script = "false; ``; echo $?\nfalse; $(); echo $?\nfalse; $(   ); echo $?\nfalse; a=``; echo $?\nfalse; a=$(); echo $?\ntrue; a=$(exit 3); echo $?\n"
	if stdout, status := runScriptCapturing(script); stdout != "0\n0\n0\n0\n0\n3\n" || status != 0 {
		t.Errorf("got %q/%d, want what busybox and bash print", stdout, status)
	}
}
