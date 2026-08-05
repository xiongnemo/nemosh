package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_persistsLeadingAssignmentAndExport_whenDirectExportRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "X=temporary export Y=kept\necho $X:$Y\nprintenv Y\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "temporary:kept\nkept\n" {
		t.Fatalf("expected persistent export output %q, got %q", "temporary:kept\nkept\n", got)
	}
}

func TestRuntime_persistsLeadingAssignmentAndRemoval_whenDirectUnsetRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "Y=old\nexport Y\nX=temporary unset Y\necho $X:$Y\nprintenv Y || echo removed\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "temporary:\nremoved\n" {
		t.Fatalf("expected persistent unset output %q, got %q", "temporary:\nremoved\n", got)
	}
}

func TestRuntime_persistsLeadingAssignmentValueAndReadonlyFlag_whenDirectReadonlyRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "X=temporary readonly Y=value\necho $X:$Y\nY=changed || echo readonly\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "temporary:value\nreadonly\n" {
		t.Fatalf("expected persistent readonly output %q, got %q", "temporary:value\nreadonly\n", got)
	}
}

func TestRuntime_restoresLeadingAssignmentButKeepsWorkingDirectory_whenCdRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatalf("expected target directory creation to succeed, got %v", err)
	}
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{
		Cwd: runtime.WorkingDirectory(root),
		Env: runtime.NewEnvironment(nil),
	})

	// When
	status := rt.RunScript(context.Background(), "X=original\nX=temporary cd target\necho $X\npwd\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 || lines[0] != "original" || !strings.HasSuffix(lines[1], "/target") {
		t.Fatalf("expected restored assignment and target pwd, got %q", stdout.String())
	}
	if got := rt.WorkingDirectory(); got != displayPath(target) {
		t.Fatalf("expected runtime cwd %q, got %q", displayPath(target), got)
	}
}

func TestRuntime_restoresLeadingAssignment_whenRegularBuiltinRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "X=original\nX=temporary echo $X\necho $X\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "original\noriginal\n" {
		t.Fatalf("expected regular builtin assignment output %q, got %q", "original\noriginal\n", got)
	}
}

func TestRuntime_builtinOperationWins_whenItMutatesLeadingAssignmentVariable(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "X=original\nX=temporary command export X=builtin\necho $X\nprintenv X\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "builtin\nbuiltin\n" {
		t.Fatalf("expected builtin mutation output %q, got %q", "builtin\nbuiltin\n", got)
	}
}

func TestRuntime_commandExportRestoresLeadingAssignmentAndPersistsExport(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "X=original\nX=temporary command export Y=kept\necho $X:$Y\nprintenv Y\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "original:kept\nkept\n" {
		t.Fatalf("expected command export output %q, got %q", "original:kept\nkept\n", got)
	}
}

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
