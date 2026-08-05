package main

import (
	"strings"
	"testing"
)

func TestRunInteractive_preservesPhysicalNewlineInsideQuotedValue(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("echo \"left |\nright\"\nexit 0\n"))

	// Then
	if got.err != nil {
		t.Fatalf("interactive run: %v; stderr = %q", got.err, got.stderr)
	}
	if got.stdout != "left |\nright\n" {
		t.Fatalf("stdout = %q, want %q", got.stdout, "left |\nright\n")
	}
}

func TestRunInteractive_preservesPhysicalNewlineInsideHeredocBody(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("cat <<EOF\nbody |\nEOF\nexit 0\n"))

	// Then
	if got.err != nil {
		t.Fatalf("interactive run: %v; stderr = %q", got.err, got.stderr)
	}
	if got.stdout != "body |\n" {
		t.Fatalf("stdout = %q, want %q", got.stdout, "body |\n")
	}
}
