package runtime_test

import (
	"strings"
	"testing"
)

// `break n` and `continue n` used to parse the operand and discard it, so `break 2`
// broke one loop and the outer one carried on. Nothing failed and nothing was
// printed, which is the same shape the `read -r` defect had: the script runs and
// does something other than what it says.
//
// Every expectation is measured against bash, and the three cases where all of
// bash, dash and busybox ash agree say so.
func TestLoopLevels_breakAndContinueTakeACount(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			// The case that was wrong: `inner` must not print, because the outer
			// loop is broken too.
			name:   "break 2 leaves both loops",
			script: "for a in 1 2; do for b in 1 2; do break 2; done; echo inner; done\necho out\n",
			want:   "out\n",
		},
		{
			name:   "break 1 is the bare break",
			script: "for a in 1 2; do for b in x y; do break 1; done; echo inner; done\n",
			want:   "inner\ninner\n",
		},
		{
			name:   "a bare break leaves one loop",
			script: "for a in 1 2; do for b in x y; do break; done; echo inner; done\n",
			want:   "inner\ninner\n",
		},
		{
			// Three deep, breaking two: the outermost body resumes after the
			// middle loop, so `outer` prints once and `mid` never does.
			name:   "break 2 from three deep leaves the outermost running",
			script: "for a in 1; do for b in 1; do for c in 1 2; do break 2; done; echo mid; done; echo outer; done\n",
			want:   "outer\n",
		},
		{
			// continue 2 must resume the *outer* loop, so the outer body after the
			// inner loop is skipped and the outer loop keeps iterating.
			name:   "continue 2 resumes the outer loop",
			script: "for a in 1 2 3; do for b in x y; do continue 2; done; echo never; done\necho out\n",
			want:   "out\n",
		},
		{
			name:   "continue 1 is the bare continue",
			script: "for a in 1 2; do for b in x; do continue 1; done; echo inner; done\n",
			want:   "inner\ninner\n",
		},
		{
			// A count past the nesting stops at the outermost loop rather than
			// carrying the break into the rest of the script. Without the clamp
			// this printed nothing.
			name:   "break past the nesting stops at the outermost loop",
			script: "for a in 1 2; do break 5; done\necho out\n",
			want:   "out\n",
		},
		{
			name:   "continue past the nesting does the same",
			script: "for a in 1 2; do continue 9; done\necho out\n",
			want:   "out\n",
		},
		{
			// The count must not survive the loop it was used in.
			name:   "a later loop is unaffected",
			script: "for a in 1 2; do break 2; done\nfor b in 1 2; do echo \"second=$b\"; done\n",
			want:   "second=1\nsecond=2\n",
		},
		{
			name:   "while takes a count too",
			script: "i=0\nwhile [ $i -lt 3 ]; do i=$((i+1)); for b in x; do break 2; done; echo inner; done\necho \"i=$i\"\n",
			want:   "i=1\n",
		},
		{
			name:   "until takes a count too",
			script: "i=0\nuntil [ $i -ge 3 ]; do i=$((i+1)); while true; do break 2; done; echo inner; done\necho \"i=$i\"\n",
			want:   "i=1\n",
		},
		{
			// All three reference shells continue here and all exit 0.
			name:   "outside a loop it does nothing and the script goes on",
			script: "break\necho after\n",
			want:   "after\n",
		},
		{
			name:   "continue outside a loop likewise",
			script: "continue\necho after\n",
			want:   "after\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// A count that is not a count has to say so. Silently treating `break abc` as
// `break` is how the original defect looked from the outside.
func TestLoopLevels_refusesACountThatIsNotOne(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		fragment string
	}{
		{
			name:   "not a number",
			script: "for a in 1; do break abc; done\n", fragment: "numeric argument required",
		},
		{
			name:   "zero would mean breaking no loops",
			script: "for a in 1; do break 0; done\n", fragment: "loop count out of range",
		},
		{
			name:   "negative",
			script: "for a in 1; do continue -1; done\n", fragment: "loop count out of range",
		},
		{
			name:   "continue names itself in the diagnostic",
			script: "for a in 1; do continue xyz; done\n", fragment: "continue: xyz",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, _, stderr := runSetScript(t, test.script)

			// Then
			if !strings.Contains(stderr, test.fragment) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, test.fragment)
			}
		})
	}
}

// A subshell has no loop of its own, so a break inside one cannot reach the loop
// outside it -- the subshell is a separate execution and the loop is not in it.
func TestLoopLevels_doNotCrossASubshell(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "for a in 1 2 3; do (break 2); echo \"body=$a\"; done\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if want := "body=1\nbody=2\nbody=3\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q -- the break must stay inside the subshell", stdout, want)
	}
}
