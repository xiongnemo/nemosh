package main

import (
	"strings"
	"testing"
)

// At a prompt the line `fc -l` is recorded before it runs, and fc deals in the commands
// before it, as bash's last_hist has it: `fc -l` lists the two echoes and not itself, and
// `fc -l -1` the one before, which is that fc. Measured in bash 5.3.
func TestFc_atAPromptDealsInTheCommandsBeforeIt(t *testing.T) {
	got := runInteractiveTest(strings.NewReader("echo 1\necho 2\nfc -l\nfc -l -1\n"))
	if want := "1\n2\n1\t echo 1\n2\t echo 2\n3\t fc -l\n"; got.stdout != want {
		t.Fatalf("stdout = %q, want %q", got.stdout, want)
	}
}

// fc -s runs the command before it again and takes its place in the list. bash keeps both
// echoes; this list keeps no entry twice in a row, so it has the one.
func TestFc_sAtAPromptTakesItsOwnPlace(t *testing.T) {
	got := runInteractiveTest(strings.NewReader("echo hi\nfc -s\nhistory\n"))
	if want := "hi\nhi\n1  echo hi\n2  history\n"; got.stdout != want {
		t.Fatalf("stdout = %q, want %q", got.stdout, want)
	}
	if !strings.Contains(got.stderr, "echo hi\n") {
		t.Fatalf("stderr = %q, want fc to say the command it runs", got.stderr)
	}
}
