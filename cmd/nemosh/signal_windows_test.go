//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestWindowsInterruptBoundaryDoesNotClaimTargetedChildCtrlC(t *testing.T) {
	if !strings.Contains(windowsInterruptBoundary, "cannot direct os.Interrupt") {
		t.Fatalf("boundary = %q", windowsInterruptBoundary)
	}
}
