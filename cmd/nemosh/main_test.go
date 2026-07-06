package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestRun_returnsNil_whenDispatchingTrueApplet(t *testing.T) {
	// Given
	args := []string{"nemosh", "true"}

	// When
	err := run(context.Background(), args)

	// Then
	if err != nil {
		t.Fatalf("expected true applet to succeed, got %v", err)
	}
}

func TestRun_returnsFalseSentinel_whenDispatchingFalseApplet(t *testing.T) {
	// Given
	args := []string{"nemosh", "false"}

	// When
	err := run(context.Background(), args)

	// Then
	if !errors.Is(err, applets.ErrExitFalse) {
		t.Fatalf("expected false sentinel, got %v", err)
	}
}

func TestRun_writesEchoOutput_whenDispatchingEchoApplet(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	cmd := command{stdin: &bytes.Buffer{}, stdout: &stdout, stderr: &bytes.Buffer{}}

	// When
	err := cmd.run(context.Background(), []string{"nemosh", "echo", "hello"})

	// Then
	if err != nil {
		t.Fatalf("expected echo applet to succeed, got %v", err)
	}
	if got := stdout.String(); got != "hello\n" {
		t.Fatalf("expected stdout %q, got %q", "hello\n", got)
	}
}

func TestRun_executesScript_whenCommandFlagProvided(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	cmd := command{stdin: &bytes.Buffer{}, stdout: &stdout, stderr: &bytes.Buffer{}}

	// When
	err := cmd.run(context.Background(), []string{"nemosh", "-c", "echo hi"})

	// Then
	if err != nil {
		t.Fatalf("expected -c script to succeed, got %v", err)
	}
	if got := stdout.String(); got != "hi\n" {
		t.Fatalf("expected stdout %q, got %q", "hi\n", got)
	}
}

func TestRun_executesStdinScript_whenNoAppletProvided(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	cmd := command{stdin: bytes.NewBufferString("echo from-stdin\n"), stdout: &stdout, stderr: &bytes.Buffer{}}

	// When
	err := cmd.run(context.Background(), []string{"nemosh"})

	// Then
	if err != nil {
		t.Fatalf("expected stdin script to succeed, got %v", err)
	}
	if got := stdout.String(); got != "from-stdin\n" {
		t.Fatalf("expected stdout %q, got %q", "from-stdin\n", got)
	}
}

func TestRun_executesInteractiveInput_whenInteractiveFlagProvided(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	cmd := command{stdin: bytes.NewBufferString("echo interactive\nexit 0\n"), stdout: &stdout, stderr: &bytes.Buffer{}}

	// When
	err := cmd.run(context.Background(), []string{"nemosh", "-i"})

	// Then
	if err != nil {
		t.Fatalf("expected interactive input to succeed, got %v", err)
	}
	if got := stdout.String(); got != "$ interactive\n$ " {
		t.Fatalf("expected REPL transcript %q, got %q", "$ interactive\n$ ", got)
	}
}
