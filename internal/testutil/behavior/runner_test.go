package behavior_test

import (
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/testutil/behavior"
)

func TestRunner_executesCommandCase_whenAppletExists(t *testing.T) {
	// Given
	caseData := behavior.Case{
		Command: []string{"echo", "hello", "world"},
		Expect:  behavior.Expect{Status: 0, Stdout: "hello world\n", Stderr: ""},
	}
	runner := behavior.NewRunner(applets.DefaultRegistry)

	// When
	result := runner.Run(context.Background(), caseData)

	// Then
	if result.Status != caseData.Expect.Status {
		t.Fatalf("expected status %d, got %d", caseData.Expect.Status, result.Status)
	}
	if result.Stdout != caseData.Expect.Stdout {
		t.Fatalf("expected stdout %q, got %q", caseData.Expect.Stdout, result.Stdout)
	}
	if result.Stderr != caseData.Expect.Stderr {
		t.Fatalf("expected stderr %q, got %q", caseData.Expect.Stderr, result.Stderr)
	}
}

func TestRunner_returnsCommandNotFound_whenAppletMissing(t *testing.T) {
	// Given
	caseData := behavior.Case{Command: []string{"missing-applet"}}
	runner := behavior.NewRunner(applets.DefaultRegistry)

	// When
	result := runner.Run(context.Background(), caseData)

	// Then
	if result.Status != 127 {
		t.Fatalf("expected status 127, got %d", result.Status)
	}
	if result.Stderr != "missing-applet: not found\n" {
		t.Fatalf("expected not found stderr, got %q", result.Stderr)
	}
}

func TestRunner_returnsAppletStatus_whenAppletReportsStatusCode(t *testing.T) {
	// Given
	caseData := behavior.Case{Command: []string{"sort", "-z"}}
	runner := behavior.NewRunner(applets.DefaultRegistry)

	// When
	result := runner.Run(context.Background(), caseData)

	// Then
	if result.Status != 2 {
		t.Fatalf("expected status 2, got %d", result.Status)
	}
	if result.Stdout != "" {
		t.Fatalf("expected empty stdout, got %q", result.Stdout)
	}
	if result.Stderr != "sort: invalid option -- z\n" {
		t.Fatalf("expected invalid option stderr, got %q", result.Stderr)
	}
}
