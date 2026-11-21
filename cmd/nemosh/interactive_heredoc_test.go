package main

import (
	"strings"
	"testing"
)

func TestRunInteractive_collectsPendingHeredocBeforeExecution(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("cat <<EOF\ninteractive\nEOF\n"))

	// Then
	if got.stdout != "interactive\n" || interactiveStatus(t, got.err) != 0 {
		t.Fatalf("outcome = %+v, want heredoc output and status 0", got)
	}
	if strings.Count(withoutANSI(got.stderr), "> ") != 2 {
		t.Fatalf("stderr = %q, want two continuation prompts", got.stderr)
	}
}

// A here-document's operator continued onto the next line, `cat <<\`, takes its delimiter from
// that line, at a prompt as in a script: both references say body and after. A session took
// the line it had read for all there was, and refused `cat <<` with no word. Measured 2026-11-22
// with Git for Windows' bash 5.3 and busybox-w32, both -i.
func TestRunInteractive_heredocOperatorContinuedOntoTheNextLine(t *testing.T) {
	got := runInteractiveTest(strings.NewReader("cat <<\\\nEOF\nbody\nEOF\necho after\n"))
	if got.stdout != "body\nafter\n" || strings.Contains(got.stderr, "syntax error") {
		t.Fatalf("outcome = %+v, want body and after", got)
	}
}

func TestRunInteractive_incompleteHeredocEOFReportsOnceAndReturnsTwo(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("cat <<EOF\nbody\n"))

	// Then
	if status := interactiveStatus(t, got.err); status != 2 {
		t.Fatalf("status = %d, want 2", status)
	}
	if got.stdout != "" || strings.Count(got.stderr, "nemosh: unexpected end of file") != 1 {
		t.Fatalf("outcome = %+v, want no output and one EOF diagnostic", got)
	}
}
