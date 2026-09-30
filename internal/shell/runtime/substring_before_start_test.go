package runtime_test

import "testing"

// A substring whose negative offset reaches before the start of the value is empty, as busybox
// and bash have it: `${1: -8}` of seven characters. It was clamped to the start and gave the
// whole value. busybox's ash_test var_bash1b.
func TestRuntime_substringBeforeTheStartIsEmpty(t *testing.T) {
	const script = "set -- 0123456; echo \"[${1: -8}] [${1: -7}] [${1: -9:2}] [${1:8}] [${1:7}] [${1: -3:2}] [${1:2:-1}]\"\n"
	if stdout, status := runScriptCapturing(script); stdout != "[] [0123456] [] [] [] [45] [2345]\n" || status != 0 {
		t.Errorf("got %q/%d, want what busybox and bash print", stdout, status)
	}
}
