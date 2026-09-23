package runtime_test

import "testing"

// `trap ... ERR` runs after a command fails where `set -e` would act. It is not a signal, so
// nothing Windows lacks stands in its way; it was refused as one. Every transcript below is
// the answer busybox-w32 and bash both give, except the last, where they part and bash's is
// kept -- see the case.
func TestTrapERR_runsWhereSetEWouldAct(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "after a failure, with its status", script: "trap 'echo err $?' ERR\nfalse\necho after\n", want: "err 1\nafter\n"},
		{name: "not in a condition, a list or under !", script: "trap 'echo err' ERR\nif false; then :; fi\nfalse || true\nfalse && true\n! false\nwhile false; do :; done\necho quiet\n", want: "quiet\n"},
		{name: "a pipeline's status is its last stage's", script: "trap 'echo err' ERR\nfalse | true\ntrue | false\necho pipes\n", want: "err\npipes\n"},
		{name: "not inside a function, once for its status", script: "trap 'echo err' ERR\nf() { false; echo in-f; return 3; }\nf\necho after\n", want: "in-f\nerr\nafter\n"},
		{name: "set -E carries it into a function", script: "set -E\ntrap 'echo err' ERR\nf() { false; echo in-f; }\nf\n", want: "err\nin-f\n"},
		{name: "once for a subshell's status", script: "trap 'echo err' ERR\n(false)\nx=$(false)\necho done\n", want: "err\nerr\ndone\n"},
		{name: "before set -e exits", script: "set -e\ntrap 'echo err' ERR\nfalse\necho not-reached\n", want: "err\n"},
		{name: "reset", script: "trap 'echo err' ERR\ntrap - ERR\nfalse\necho reset\n", want: "reset\n"},
		{name: "listed", script: "trap 'echo err' ERR\ntrap\n", want: "trap -- 'echo err' ERR\n"},
		// Here the references part: bash runs the trap inside the subshell too, as its own
		// documentation of errtrace says, and busybox-w32 does not -- its forkshell clears
		// every child's traps. errtrace is bash's option, so bash's reading is the one kept.
		{name: "set -E carries it into a subshell", script: "set -E\ntrap 'echo err' ERR\n(false; echo in-sub)\n", want: "err\nin-sub\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}
