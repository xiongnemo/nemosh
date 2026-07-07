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
