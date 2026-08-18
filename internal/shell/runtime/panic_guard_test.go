package runtime

import (
	"bytes"
	"strings"
	"testing"
)

// The guards are tested by panicking on purpose, because the only way to know a
// recover is in the right place is to make something panic there.
//
// This is an internal test so it can call the guards directly. Reaching them
// through a real defect would mean keeping a defect around, and the four
// placements are what the test is about -- the recover in main could never have
// caught the background-goroutine panic that prompted all of this.

func guardTestRuntime(stderr *bytes.Buffer) Runtime {
	created := Runtime{
		streams:   Streams{Stderr: stderr},
		vars:      map[string]string{},
		expansion: newExpansionState(),
	}
	return created
}

func TestGuardedStatus_reportsAPanicAsAFailingStatus(t *testing.T) {
	// Given
	var stderr bytes.Buffer
	r := guardTestRuntime(&stderr)

	// When
	status := r.guardedStatus("running a command", func() int {
		panic("a deliberate defect")
	})

	// Then
	if status != internalErrorStatus {
		t.Fatalf("status = %d, want %d so that `set -e` and `||` can react", status, internalErrorStatus)
	}
	if !strings.Contains(stderr.String(), "internal error while running a command") {
		t.Fatalf("stderr = %q, want it to name what was running", stderr.String())
	}
	if !strings.Contains(stderr.String(), "a deliberate defect") {
		t.Fatalf("stderr = %q, want the panic value", stderr.String())
	}
}

// The first line has to say this is nemosh's fault, not the user's. Someone whose
// script just died needs to know which of the two to go and look at.
func TestGuardedStatus_saysTheDefectIsInTheShell(t *testing.T) {
	// Given
	var stderr bytes.Buffer
	r := guardTestRuntime(&stderr)

	// When
	r.guardedStatus("running a command", func() int { panic("boom") })

	// Then
	output := stderr.String()
	if !strings.Contains(output, "defect in nemosh, not in what you typed") {
		t.Fatalf("stderr = %q, want it to place the blame", output)
	}
	if !strings.Contains(output, "NEMOSH_DEBUG=panic") {
		t.Fatalf("stderr = %q, want it to say how to get the trace", output)
	}
}

// The trace is off by default and on when asked for. Off matters because the
// behavior corpus compares output byte for byte and a trace carries host paths.
func TestGuardedStatus_printsTheTraceOnlyWhenAsked(t *testing.T) {
	for _, test := range []struct {
		name      string
		debug     string
		wantTrace bool
	}{
		{name: "unset", debug: "", wantTrace: false},
		{name: "another channel", debug: "path,exec", wantTrace: false},
		{name: "the panic channel", debug: "panic", wantTrace: true},
		{name: "among others", debug: "path,panic", wantTrace: true},
		{name: "all", debug: "all", wantTrace: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			var stderr bytes.Buffer
			r := guardTestRuntime(&stderr)
			if test.debug != "" {
				r.vars["NEMOSH_DEBUG"] = test.debug
			}

			// When
			r.guardedStatus("running a command", func() int { panic("boom") })

			// Then
			hasTrace := strings.Contains(stderr.String(), "panic_guard_test.go")
			if hasTrace != test.wantTrace {
				t.Fatalf("trace present = %v, want %v; stderr = %q",
					hasTrace, test.wantTrace, stderr.String())
			}
		})
	}
}

// A nil panic value must not be mistaken for no panic. `panic(nil)` became a
// runtime.PanicNilError in Go 1.21 rather than an untyped nil, but the guard should
// not depend on that.
func TestGuardedRun_survivesANilPanicValue(t *testing.T) {
	// Given
	var stderr bytes.Buffer
	r := guardTestRuntime(&stderr)

	// When
	result := r.guardedRun("running a pipeline stage", func() lineResult {
		panic(nil)
	})

	// Then
	if result.status != internalErrorStatus {
		t.Fatalf("status = %d, want %d", result.status, internalErrorStatus)
	}
}

// A guard that does not run the work is worse than no guard.
func TestGuardedRun_returnsTheResultWhenNothingPanics(t *testing.T) {
	// Given
	var stderr bytes.Buffer
	r := guardTestRuntime(&stderr)

	// When
	result := r.guardedRun("running a command", func() lineResult {
		return lineResult{status: 3}
	})

	// Then
	if result.status != 3 {
		t.Fatalf("status = %d, want the work's own 3", result.status)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want nothing", stderr.String())
	}
}
