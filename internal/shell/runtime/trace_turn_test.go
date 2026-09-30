package runtime_test

import "testing"

// set -x traces a pipeline's stages in the order busybox-w32 writes them, each order below
// measured there: a stage's lines until it starts a program or forks, then the next stage's,
// and what a stage forks once the last stage has begun. bash 5.3 writes the same, but for a
// substitution's line, which it puts after the next stage's first line rather than the last
// stage's. The stages are goroutines, and the lines came in whichever order they were
// reached; each script runs several times so that an order left to chance shows.
func TestTrace_pipelineStagesInOrder(t *testing.T) {
	for _, test := range []struct{ name, script, want string }{
		{name: "a stage at a time", script: "echo a | cat | tr a b", want: "+ echo a\n+ cat\n+ tr a b\n"},
		{name: "a function's body before the next stage", script: "g() { echo 1; echo 2; }\ng | cat", want: "+ g\n+ echo 1\n+ echo 2\n+ cat\n"},
		{name: "builtins keep the turn", script: "eval 'echo a' | cat", want: "+ eval 'echo a'\n+ echo a\n+ cat\n"},
		{name: "a program passes it on", script: "{ cat /dev/null; echo b; } | cat", want: "+ cat /dev/null\n+ cat\n+ echo b\n"},
		{name: "a substitution after the stages", script: "x=$(echo sub) | cat | cat", want: "+ cat\n+ cat\n+ echo sub\n+ x=sub\n"},
		{name: "a substitution in the last stage", script: "echo a | x=$(echo sub)", want: "+ echo a\n+ echo sub\n+ x=sub\n"},
		{name: "a subshell that is the stage", script: "echo a | (cat) | tr a b", want: "+ echo a\n+ cat\n+ tr a b\n"},
		{name: "a pipeline inside a stage", script: "echo a | (cat | cat) | tr a b", want: "+ echo a\n+ tr a b\n+ cat\n+ cat\n"},
		{name: "a read of what is there", script: "echo a | { read x; echo $x; } | cat", want: "+ echo a\n+ read x\n+ echo a\n+ cat\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			for range 5 {
				_, _, stderr := runSetScript(t, "set -x\n"+test.script+"\nset +x\n")
				if want := test.want + "+ set +x\n"; stderr != want {
					t.Fatalf("stderr = %q, want %q", stderr, want)
				}
			}
		})
	}
}

// A stage that waits in a builtin keeps no one waiting on its turn: the read below waits for
// a substitution in the stage before it, whose lines wait for the last stage, and the last
// stage for the read's. It ends, and in busybox's order.
func TestTrace_aWaitingStageHoldsNoOneUp(t *testing.T) {
	// When
	_, stdout, stderr := runSetScript(t, "set -x\n{ x=$(echo sub); echo $x; } | { read y; echo $y; } | cat\nset +x\n")

	// Then
	want := "+ read y\n+ cat\n+ echo sub\n+ x=sub\n+ echo sub\n+ echo sub\n+ set +x\n"
	if stdout != "sub\n" || stderr != want {
		t.Fatalf("stdout = %q, stderr = %q, want %q and %q", stdout, stderr, "sub\n", want)
	}
}
