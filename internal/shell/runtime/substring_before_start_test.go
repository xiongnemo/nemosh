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

// A colon with nothing after it is no substring, and both references refuse the expansion:
// bash calls `${v:}` a bad substitution and busybox a missing `}`. It was the whole value. An
// offset written empty is 0 in both, as `${v::2}` and `${v:$e}`. busybox's ash_test
// param_expand_bash_substring.
func TestRuntime_aSubstringWithNoOffsetIsRefused(t *testing.T) {
	stdout, status := runScriptCapturing("v=0123\necho \"${v:}\"\necho after\n")
	if stdout != "" || status != 2 {
		t.Errorf("got %q/%d, want no output and 2", stdout, status)
	}
	const written = "v=0123\ne=\necho \"[${v::2}] [${v:$e}] [${v:1:}]\"\n"
	if stdout, status := runScriptCapturing(written); stdout != "[01] [0123] []\n" || status != 0 {
		t.Errorf("got %q/%d, want what busybox and bash print", stdout, status)
	}
}
