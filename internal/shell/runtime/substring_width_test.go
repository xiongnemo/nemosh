package runtime_test

import "testing"

// A substring's offset and length are bash's intmax_t, 64 bits on every build, as an array's
// index is (array_sparse_store_test.go). They were an int, 32 bits on windows/386, so there
// `${x:4294967297}` was `${x:1}`, `${x:1:4294967297}` one character, and an array sliced the
// same way. The answers are bash 5.3's.
func TestRuntime_substringOffsetsAreSixtyFourBits(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"x=abcdef; echo \"[${x:4294967297}]\" \"[${x:1:4294967297}]\"\n", "[] [bcdef]\n"},
		{"a=(p q r s); echo \"[${a[@]:4294967297}]\" \"[${a[@]:1:4294967297}]\"\n", "[] [q r s]\n"},
		// On every build, a length near the largest an int64 holds was added to the offset and
		// wrapped, which made it nothing. bash means the rest of the value, and tries to allocate
		// 2^63 bytes for it.
		{"x=abcdef; echo \"[${x:1:9223372036854775807}]\"\n", "[bcdef]\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
