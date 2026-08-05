package main

import (
	"errors"
	"strings"
	"testing"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("broken input") }

func TestRunInteractive_wrapsNonEOFReadError(t *testing.T) {
	// Given / When
	got := runInteractiveTest(failingReader{})

	// Then
	if got.err == nil || got.err.Error() != "nemosh: read stdin: broken input" {
		t.Fatalf("error = %v, want wrapped stdin error", got.err)
	}
}

func TestRunInteractive_rejectsOversizedPhysicalLineWithoutNewline(t *testing.T) {
	// Given
	limit := parserInputLimit(t)
	input := strings.NewReader(strings.Repeat("x", limit+1))

	// When
	got := runInteractiveTest(input)

	// Then
	if status := interactiveStatus(t, got.err); status != 2 {
		t.Fatalf("status = %d, want 2", status)
	}
	if !strings.Contains(got.stderr, "input too large") {
		t.Fatalf("stderr = %q, want input-too-large diagnostic", got.stderr)
	}
}

func TestRunInteractive_rejectsOversizedCumulativeMultilineInput(t *testing.T) {
	// Given
	limit := parserInputLimit(t)
	line := strings.Repeat("x", limit/2) + "\\\n"
	input := strings.NewReader(line + line + "x\n")

	// When
	got := runInteractiveTest(input)

	// Then
	if status := interactiveStatus(t, got.err); status != 2 {
		t.Fatalf("status = %d, want 2", status)
	}
	if !strings.Contains(got.stderr, "input too large") {
		t.Fatalf("stderr = %q, want input-too-large diagnostic", got.stderr)
	}
}
