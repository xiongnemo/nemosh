package runtime_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_keepsStandaloneAssignmentPersistent_whenLineContainsOnlyAssignment(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "name=nemo\necho $name\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "nemo\n" {
		t.Fatalf("expected stdout %q, got %q", "nemo\n", got)
	}
}

func TestRuntime_appliesLeadingAssignmentToCommandEnvironment_whenCommandRuns(t *testing.T) {
	// Given
	name := "NEMOSH_TEST_LEADING_ASSIGNMENT"
	t.Setenv(name, "")
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("expected env setup to succeed, got %v", err)
	}
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), name+"=temporary printenv "+name+"\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "temporary\n" {
		t.Fatalf("expected leading assignment output %q, got %q", "temporary\n", got)
	}
}

func TestRuntime_doesNotPersistLeadingAssignment_whenCommandFinishes(t *testing.T) {
	// Given
	name := "NEMOSH_TEST_LEADING_ASSIGNMENT_SCOPE"
	t.Setenv(name, "")
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("expected env setup to succeed, got %v", err)
	}
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), name+"=temporary printenv "+name+"\necho $"+name+"\nprintenv "+name+"\n")

	// Then
	if status != 1 {
		t.Fatalf("expected final printenv status 1, got %d", status)
	}
	if got := stdout.String(); got != "temporary\n\n" {
		t.Fatalf("expected scoped assignment output %q, got %q", "temporary\n\n", got)
	}
}

func TestRuntime_EnvAppletDoesNotPersistAssignment_whenEnvRunsCommand(t *testing.T) {
	// Given
	name := "NEMOSH_TEST_ENV_APPLET_RUNTIME_ASSIGNMENT"
	t.Setenv(name, "")
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("expected env setup to succeed, got %v", err)
	}
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "env "+name+"=temporary printenv "+name+"\necho $"+name+"\nprintenv "+name+"\n")

	// Then
	if status != 1 {
		t.Fatalf("expected final printenv status 1, got %d", status)
	}
	if got := stdout.String(); got != "temporary\n\n" {
		t.Fatalf("expected env applet assignment output %q, got %q", "temporary\n\n", got)
	}
}
