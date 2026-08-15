package main

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
)

// -N has to come off the front before anything reads args by position, and it
// must take nothing else with it. `nemosh -N -c CMD` is the shape su launches,
// and if -N were left in place the chain would see it as args[1] and answer
// `invalid option`.
func TestStripHoldOption(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
		held bool
	}{
		{
			name: "the shape su launches",
			args: []string{"nemosh", "-N", "-c", "ls"},
			want: []string{"nemosh", "-c", "ls"}, held: true,
		},
		{
			name: "before an interactive shell",
			args: []string{"nemosh", "-N", "-i"},
			want: []string{"nemosh", "-i"}, held: true,
		},
		{
			name: "alone",
			args: []string{"nemosh", "-N"},
			want: []string{"nemosh"}, held: true,
		},
		{
			// The whole reason this only reads the leading run: -N after the
			// first other word belongs to whoever that word was for.
			name: "not an argument of -c",
			args: []string{"nemosh", "-c", "echo -N"},
			want: []string{"nemosh", "-c", "echo -N"}, held: false,
		},
		{
			name: "not a script's operand",
			args: []string{"nemosh", "run.sh", "-N"},
			want: []string{"nemosh", "run.sh", "-N"}, held: false,
		},
		{
			name: "nothing at all",
			args: []string{"nemosh"},
			want: []string{"nemosh"}, held: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got, held := stripHoldOption(test.args)

			// Then
			if !slices.Equal(got, test.want) || held != test.held {
				t.Fatalf("stripHoldOption(%q) = (%q, %v), want (%q, %v)", test.args, got, held, test.want, test.held)
			}
		})
	}
}

// Stripping must not scribble on the caller's slice. os.Args is what production
// passes, and a `-N` run that shifted the words in place would leave the process
// holding an argv it never had.
func TestStripHoldOption_leavesTheCallersSliceAlone(t *testing.T) {
	// Given
	args := []string{"nemosh", "-N", "-c", "ls"}

	// When
	if _, held := stripHoldOption(args); !held {
		t.Fatal("expected -N to be found")
	}

	// Then
	if !slices.Equal(args, []string{"nemosh", "-N", "-c", "ls"}) {
		t.Fatalf("args = %q, want them untouched", args)
	}
}

// Holding waits for a keypress, and with no terminal there is nobody to press
// one. Waiting anyway would hang a pipeline forever, which is a worse failure
// than the one -N exists to prevent -- and a redirected stdin means the output
// was captured, so there was nothing to lose.
func TestRun_doesNotHoldWhenThereIsNoTerminal(t *testing.T) {
	// Given
	var stdout, stderr bytes.Buffer
	cmd := command{
		stdin:           strings.NewReader(""),
		stdout:          &stdout,
		stderr:          &stderr,
		stdinIsTerminal: false,
	}

	// When: this returns only if the hold was skipped.
	err := cmd.run(context.Background(), []string{"nemosh", "-N", "-c", "echo held"})

	// Then
	if err != nil {
		t.Fatalf("run = %v", err)
	}
	if stdout.String() != "held\n" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "held\n")
	}
	if strings.Contains(stderr.String(), "Press any key") {
		t.Fatalf("stderr = %q, want no prompt with no terminal to answer it", stderr.String())
	}
}
