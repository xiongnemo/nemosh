package runtime_test

import "testing"

// A break or continue in a subshell inside a loop ends the subshell, as busybox's forked
// subshell has it, and leaves the loop outside alone: a command substitution and a pipeline's
// stage are subshells too. They were ignored and the rest of the subshell ran; bash says the
// break is meaningless there and runs the rest too, and busybox decides. busybox's ash_test
// break5.
func TestRuntime_breakInASubshellEndsIt(t *testing.T) {
	const script = "for i in 1 2; do x=$(echo pre; break; echo in); echo \"[$x]\"; done\n" +
		"for v in a b; do (echo B; break; echo C); echo \"D st=$?\"; done\n" +
		"for v in a; do echo x | { break; echo no; }; echo \"after pipe $?\"; done\n" +
		"while true; do (continue; echo C2); echo D2; break; done\n" +
		"(break; echo top); echo end\n"
	const want = "[pre]\n[pre]\nB\nD st=0\nB\nD st=0\nafter pipe 0\nD2\ntop\nend\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Errorf("got %q/%d, want %q/0, as busybox answers", stdout, status, want)
	}
}
