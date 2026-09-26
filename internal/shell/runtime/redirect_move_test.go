package runtime_test

import "testing"

// `n>&m-` moves a descriptor: n becomes a copy of m, and m is closed, as ksh has it and bash
// took it up. Each spelling was refused as a malformed redirection, which stopped the whole
// script before its first line. busybox-w32 does not have it, so the answers are bash 5.3's.
func TestRuntime_redirectionMovesADescriptor(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`exec 4>&1 3>&4-; echo hi >&3`, "hi\n"},
		{`exec 3>&1; exec 4>&3-; echo x >&4; { echo y >&3; } 2>/dev/null || echo closed`, "x\nclosed\n"},
		{`{ echo moved >&5; } 5>&1-`, "moved\n"},
		{`f() { echo inf >&6; }; f 6>&1-`, "inf\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
