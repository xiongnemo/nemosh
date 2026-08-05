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
	if strings.Count(got.stderr, "> ") != 2 {
		t.Fatalf("stderr = %q, want two continuation prompts", got.stderr)
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
