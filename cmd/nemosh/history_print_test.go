package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// `history -p` prints its words history-expanded against the list, as bash's does, and a
// word that does not expand is said and fails the builtin. Measured in bash 5.3.
func TestHistoryP_expandsItsWordsAgainstTheList(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := command{stdin: strings.NewReader(""), stdout: &stdout, stderr: &stderr}
	script := `history -s "echo a"; history -s "echo b c"` + "\n" +
		`history -p '!!' '!-2:0' 'x!!y' '!x' plain; echo s=$?` + "\n"

	// When
	err := cmd.run(context.Background(), []string{"nemosh", "-c", script})

	// Then
	if err != nil {
		t.Fatalf("run = %v; stderr %q", err, stderr.String())
	}
	if want := "echo b c\necho\nxecho b cy\nplain\ns=1\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if want := "history: !x: history expansion failed\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

// At a prompt, the `history -p` line is taken back out before its words are expanded, so
// `!!` is the command before it.
func TestHistoryP_atAPromptIsNotItsOwnEvent(t *testing.T) {
	got := runInteractiveTest(strings.NewReader("echo first\nhistory -p '!!'\nhistory\n"))
	if !strings.Contains(got.stdout, "first\necho first\n1  echo first\n") {
		t.Fatalf("stdout = %q, want !! to be the echo and the -p line gone", got.stdout)
	}
}
