package applets_test

import (
	"bytes"
	"context"
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
