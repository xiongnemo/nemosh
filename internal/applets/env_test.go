package applets_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestDefaultRegistry_printsEnvironment_whenEnvRunsWithoutArgs(t *testing.T) {
	// Given
	t.Setenv("NEMOSH_TEST_ENV_APPLET", "ok")
	applet, ok := applets.DefaultRegistry.Lookup("env")
	if !ok {
		t.Fatal("expected env applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected env to succeed, got %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "NEMOSH_TEST_ENV_APPLET=ok") {
		t.Fatalf("expected env output to contain test variable, got %q", got)
	}
}

func TestDefaultRegistry_printsNothing_whenEnvRunsWithIgnoreEnvironment(t *testing.T) {
	// Given
	t.Setenv("NEMOSH_TEST_ENV_APPLET", "ok")
	applet, ok := applets.DefaultRegistry.Lookup("env")
	if !ok {
		t.Fatal("expected env applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-i"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected env -i to succeed, got %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected env -i output to be empty, got %q", got)
	}
}

func TestDefaultRegistry_EnvRunsAppletWithAssignment_whenCommandProvided(t *testing.T) {
	// Given
	name := "NEMOSH_TEST_ENV_ASSIGNMENT"
	t.Setenv(name, "original")
	applet := lookupEnvApplet(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{name + "=VALUE", "printenv", name}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	if err != nil {
		t.Fatalf("expected env assignment command to succeed, got %v", err)
	}
	if got := stdout.String(); got != "VALUE\n" {
		t.Fatalf("expected assigned value output %q, got %q", "VALUE\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
	if got := os.Getenv(name); got != "original" {
		t.Fatalf("expected original environment value %q after child execution, got %q", "original", got)
	}
}

func TestDefaultRegistry_EnvRunsAppletWithCleanEnvironment_whenIgnoreEnvironmentAndAssignmentProvided(t *testing.T) {
	// Given
	name := "NEMOSH_TEST_ENV_CLEAN_ASSIGNMENT"
	t.Setenv(name, "inherited")
	applet := lookupEnvApplet(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-i", name + "=VALUE", "printenv", name}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	if err != nil {
		t.Fatalf("expected env -i assignment command to succeed, got %v", err)
	}
	if got := stdout.String(); got != "VALUE\n" {
		t.Fatalf("expected only assigned value output %q, got %q", "VALUE\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
	if got := os.Getenv(name); got != "inherited" {
		t.Fatalf("expected inherited environment value %q after clean child execution, got %q", "inherited", got)
	}
}

func TestDefaultRegistry_EnvPreservesDistinctExactCaseNames_whenRunStandalone(t *testing.T) {
	// Given
	applet := lookupEnvApplet(t)
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-i", "foo=lower", "FOO=upper", "printenv", "foo", "FOO"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected case-sensitive env lookup to succeed, got %v", err)
	}
	if got := stdout.String(); got != "lower\nupper\n" {
		t.Fatalf("expected distinct case-sensitive values, got %q", got)
	}
}

func TestDefaultRegistry_EnvAcceptsUtilityAssignmentNames_outsideShellIdentifierGrammar(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "1BAD", value: "digit"},
		{name: "A-B", value: "dash"},
		{name: "A.B", value: "dot"},
		{name: "变量", value: "unicode"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			applet := lookupEnvApplet(t)
			var stdout bytes.Buffer

			// When
			err := applet.Run(context.Background(), []string{"-i", test.name + "=" + test.value, "printenv", test.name}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

			// Then
			if err != nil {
				t.Fatalf("expected utility assignment name %q to succeed, got %v", test.name, err)
			}
			if got := stdout.String(); got != test.value+"\n" {
				t.Fatalf("expected exact lookup for %q, got %q", test.name, got)
			}
		})
	}
}

func TestDefaultRegistry_EnvRejectsEmptyAssignmentName_beforeDispatch(t *testing.T) {
	// Given
	applet := lookupEnvApplet(t)
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-i", "=value"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err == nil || err.Error() != "invalid variable name: empty" {
		t.Fatalf("expected empty variable name error, got %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected no misleading success output, got %q", got)
	}
}

func TestDefaultRegistry_EnvPreservesEmptyDuplicateAndCaseVariantAssignments(t *testing.T) {
	// Given
	applet := lookupEnvApplet(t)
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-i", "EMPTY=", "Path=title", "PATH=upper", "Path=latest", "printenv", "EMPTY", "Path", "PATH"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected assignment overlay to succeed, got %v", err)
	}
	if got := stdout.String(); got != "\nlatest\nupper\n" {
		t.Fatalf("expected empty, duplicate, and case-variant values, got %q", got)
	}
}

func TestDefaultRegistry_EnvDoesNotReclassifyCommandOperandsContainingEquals(t *testing.T) {
	// Given
	applet := lookupEnvApplet(t)
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-i", "A=value", "echo", "A=B"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected command operand to remain an argument, got %v", err)
	}
	if got := stdout.String(); got != "A=B\n" {
		t.Fatalf("expected command operand output, got %q", got)
	}
}

func TestDefaultRegistry_EnvDerivedViewPreservesParentResolvePath(t *testing.T) {
	// Given
	resolved := t.TempDir() + string(os.PathSeparator) + "parent-resolved.txt"
	if err := os.WriteFile(resolved, []byte("parent-resolved\n"), 0o600); err != nil {
		t.Fatalf("expected resolver fixture write to succeed, got %v", err)
	}
	view := delegatedPathView{resolved: resolved}
	ctx := applets.WithProcessView(context.Background(), view)
	applet := lookupEnvApplet(t)
	var stdout bytes.Buffer

	// When
	err := applet.Run(ctx, []string{"CHILD=value", "cat", "fixture"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected child applet to use parent resolver, got %v", err)
	}
	if got := stdout.String(); got != "parent-resolved\n" {
		t.Fatalf("expected delegated path %q, got %q", "parent-resolved\n", got)
	}
}

func TestDefaultRegistry_EnvRestoresEnvironment_whenChildAppletReturnsError(t *testing.T) {
	// Given
	name := "NEMOSH_TEST_ENV_ERROR_RESTORE"
	t.Setenv(name, "original")
	applet := lookupEnvApplet(t)

	// When
	err := applet.Run(context.Background(), []string{name + "=temporary", "false"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if !errors.Is(err, applets.ErrExitFalse) {
		t.Fatalf("expected false sentinel, got %v", err)
	}
	if got := os.Getenv(name); got != "original" {
		t.Fatalf("expected original environment value %q after failing child execution, got %q", "original", got)
	}
}

func TestDefaultRegistry_EnvRejectsUnsupportedOption_whenDashUProvided(t *testing.T) {
	// Given
	applet := lookupEnvApplet(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-u", "NEMOSH_TEST_ENV_ASSIGNMENT"}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	if err == nil {
		t.Fatal("expected unsupported option error")
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func lookupEnvApplet(t *testing.T) applets.Applet {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("env")
	if !ok {
		t.Fatal("expected env applet to be registered")
	}
	return applet
}

type delegatedPathView struct{ resolved string }

func (v delegatedPathView) WorkingDirectory() string        { return "generic-cwd" }
func (v delegatedPathView) Environ() []string               { return nil }
func (v delegatedPathView) LookupEnv(string) (string, bool) { return "", false }
func (v delegatedPathView) ResolvePath(string) string       { return v.resolved }
