package runtime_test

import "testing"

// Exponentiation is by squaring, as busybox's math.c and bash's ipow have it: `0**72**7` was
// ten trillion multiplications and hung the shell, found by FuzzEvaluateArithmetic. Every
// answer, wrapped at 64 bits, is both references', measured.
func TestArithmetic_raisesToAPowerBySquaring(t *testing.T) {
	script := "echo $(( 0**72**7 )) $(( 2**10 )) $(( 2**63 )) $(( 2**64 )) $(( (-3)**3 )) $(( 7**0 )) $(( 1**999999999999 )) $(( 3**40 ))\n"
	want := "0 1024 -9223372036854775808 0 -27 1 1 -6289078614652622815\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q", stdout, status, want)
	}
}
