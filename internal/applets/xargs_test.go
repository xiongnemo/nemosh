package applets_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestDefaultRegistry_XargsRunsDefaultEcho_whenInputHasWords(t *testing.T) {
	// Given
	applet := lookupXargsApplet(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, strings.NewReader("alpha beta\ngamma\t"), &stdout, &stderr)

	// Then
	if err != nil {
		t.Fatalf("expected xargs to succeed, got %v", err)
	}
	if got := stdout.String(); got != "alpha beta gamma\n" {
		t.Fatalf("expected xargs output %q, got %q", "alpha beta gamma\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestDefaultRegistry_XargsAppendsInputWords_whenCommandAndArgsProvided(t *testing.T) {
	// Given
	applet := lookupXargsApplet(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"echo", "prefix"}, strings.NewReader("one\ntwo\n"), &stdout, &stderr)

	// Then
	if err != nil {
		t.Fatalf("expected xargs to succeed, got %v", err)
	}
	if got := stdout.String(); got != "prefix one two\n" {
		t.Fatalf("expected xargs output %q, got %q", "prefix one two\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestDefaultRegistry_XargsRunsDefaultEchoOnce_whenInputIsEmpty(t *testing.T) {
	// Given
	applet := lookupXargsApplet(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, strings.NewReader(""), &stdout, &stderr)

	// Then
	if err != nil {
		t.Fatalf("expected xargs to succeed, got %v", err)
	}
	if got := stdout.String(); got != "\n" {
		t.Fatalf("expected xargs output %q, got %q", "\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestDefaultRegistry_XargsReturnsFalse_whenChildAppletReturnsFalse(t *testing.T) {
	// Given
	applet := lookupXargsApplet(t)

	// When
	err := applet.Run(context.Background(), []string{"false"}, strings.NewReader("one\n"), &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if !errors.Is(err, applets.ErrExitFalse) {
		t.Fatalf("expected false sentinel, got %v", err)
	}
}

func TestDefaultRegistry_XargsRejectsUnsupportedOption_whenDashZeroProvided(t *testing.T) {
	// Given
	applet := lookupXargsApplet(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-0"}, strings.NewReader("one\n"), &stdout, &stderr)

	// Then
	if err == nil {
		t.Fatal("expected unsupported option error")
	}
	if got := err.Error(); got != "unsupported xargs option: -0" {
		t.Fatalf("expected unsupported option error, got %q", got)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func lookupXargsApplet(t *testing.T) applets.Applet {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("xargs")
	if !ok {
		t.Fatal("expected xargs applet to be registered")
	}
	return applet
}
