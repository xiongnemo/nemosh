package main

import (
	"strings"
	"testing"
)

func TestRunInteractive_persistsStateAcrossEntries(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("name=nemo\necho $name\nexit 0\n"))

	// Then
	if status := interactiveStatus(t, got.err); status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	if got.stdout != "nemo\n" {
		t.Fatalf("stdout = %q, want %q", got.stdout, "nemo\n")
	}
}

func TestRunInteractive_continuesAfterNonzeroEntry(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("false\necho alive\nexit 0\n"))

	// Then
	if status := interactiveStatus(t, got.err); status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	if got.stdout != "alive\n" {
		t.Fatalf("stdout = %q, want %q", got.stdout, "alive\n")
	}
}

func TestRunInteractive_returnsRequestedExitStatus(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("exit 7\n"))

	// Then
	if status := interactiveStatus(t, got.err); status != 7 {
		t.Fatalf("status = %d, want 7", status)
	}
}

func TestRunInteractive_trailingBackgroundCompletesAndMalformedEntryRecovers(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("echo first &\nwait %1\necho bad & & echo nope\necho recovered\nexit 0\n"))

	// Then
	if status := interactiveStatus(t, got.err); status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	if got.stdout != "first\nrecovered\n" {
		t.Fatalf("stdout = %q, want completed background and recovery output", got.stdout)
	}
	if strings.Contains(got.stderr, "$ > ") || strings.Count(got.stderr, "nemosh:") != 1 {
		t.Fatalf("stderr = %q, want primary prompts and one malformed diagnostic", got.stderr)
	}
}

func TestRunInteractive_preservesLogicalOperatorContinuationAfterBackgroundClosure(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("echo and &&\necho continued\necho pipe |\ncat\nexit 0\n"))

	// Then
	if got.stdout != "and\ncontinued\npipe\n" || strings.Count(got.stderr, "> ") != 2 {
		t.Fatalf("outcome = %+v, want two continued entries", got)
	}
}
