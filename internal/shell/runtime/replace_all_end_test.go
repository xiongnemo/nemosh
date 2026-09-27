package runtime_test

import "testing"

// `//` stops at a match that reaches the end: the empty tail after it is no second match.
// `${v//*/-}` was `--`, where bash gives one -; busybox-w32 loops for ever on it, so bash
// decides. An empty value is still replaced once, and an empty match elsewhere once at each
// position, as before.
func TestRuntime_replaceAllStopsAtAMatchThatReachesTheEnd(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"v=abc; echo \"${v//*/-}\" \"${v//b*/-}\"", "- a-\n"},
		{"g='*'; v='a*b'; echo ${v//\"$g\"/-} ${v//$g/-}", "a-b -\n"},
		{"v=; echo \"[${v//*/-}]\"", "[-]\n"},
		{"v=abc; echo \"${v/*/-}\" \"${v//c/-}\"", "- ab-\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
