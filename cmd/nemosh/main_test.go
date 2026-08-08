package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
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

func TestRun_rejectsOversizedStdinBeforeReadingUnboundedInput(t *testing.T) {
	// Given
	limit := parserInputLimit(t)
	var stderr bytes.Buffer
	cmd := command{stdin: io.LimitReader(strings.NewReader(strings.Repeat("x", limit+1)), int64(limit+1)), stdout: &bytes.Buffer{}, stderr: &stderr}

	// When
	err := cmd.run(context.Background(), []string{"nemosh"})

	// Then
	if status := interactiveStatus(t, err); status != 2 {
		t.Fatalf("status = %d, want 2", status)
	}
	if !strings.Contains(stderr.String(), "input too large") {
		t.Fatalf("stderr = %q, want input-too-large diagnostic", stderr.String())
	}
}

func parserInputLimit(t *testing.T) int {
	t.Helper()
	low, high := 0, 1
	for runtime.InputSizeAllowed(high) {
		low, high = high, high*2
	}
	for low+1 < high {
		middle := low + (high-low)/2
		if runtime.InputSizeAllowed(middle) {
			low = middle
		} else {
			high = middle
		}
	}
	return low
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

func TestRun_returnsAppletStatus_whenDirectSortDispatchRejectsOption(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := command{stdin: &bytes.Buffer{}, stdout: &stdout, stderr: &stderr}

	// When
	err := cmd.run(context.Background(), []string{"nemosh", "sort", "-z"})

	// Then
	status, ok := applets.StatusCode(err)
	if !ok {
		t.Fatalf("expected applet status error, got %v", err)
	}
	if status != 2 {
		t.Fatalf("expected applet status 2, got %d", status)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got := stderr.String(); got != "sort: invalid option -- z\n" {
		t.Fatalf("expected invalid option stderr, got %q", got)
	}
}

func TestRun_writesWindowsPathOutput_whenDispatchingWinpathApplet(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	cmd := command{stdin: &bytes.Buffer{}, stdout: &stdout, stderr: &bytes.Buffer{}}

	// When
	err := cmd.run(context.Background(), []string{"nemosh", "winpath", "/c/tmp/a.txt"})

	// Then
	if err != nil {
		t.Fatalf("expected winpath applet to succeed, got %v", err)
	}
	if got := stdout.String(); got != "C:/tmp/a.txt\n" {
		t.Fatalf("expected stdout %q, got %q", "C:/tmp/a.txt\n", got)
	}
}

func TestRun_executesScript_whenCommandFlagProvided(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	cmd := command{
		stdin:           &bytes.Buffer{},
		stdout:          &stdout,
		stderr:          &bytes.Buffer{},
		stdinIsTerminal: true,
	}

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

func TestRun_entersInteractiveMode_whenNoArgumentsAndStdinIsTerminal(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := command{
		stdin:           bytes.NewBufferString("exit 0\n"),
		stdout:          &stdout,
		stderr:          &stderr,
		stdinIsTerminal: true,
	}

	// When
	err := cmd.run(context.Background(), []string{"nemosh"})

	// Then
	if err != nil {
		t.Fatalf("expected no-argument terminal invocation to exit cleanly, got %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected no stdout, got %q", got)
	}
	if got := withoutANSI(stderr.String()); !strings.HasPrefix(got, "# ") || !strings.HasSuffix(got, "\n"+promptSymbol()+" ") {
		t.Fatalf("expected informative default prompt, got %q", got)
	}
}

func TestRun_keepsBatchMode_whenNoArgumentsAndStdinIsRedirected(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := command{
		stdin:           bytes.NewBufferString("echo redirected\n"),
		stdout:          &stdout,
		stderr:          &stderr,
		stdinIsTerminal: false,
	}

	// When
	err := cmd.run(context.Background(), []string{"nemosh"})

	// Then
	if err != nil {
		t.Fatalf("expected redirected script to succeed, got %v", err)
	}
	if got := stdout.String(); got != "redirected\n" {
		t.Fatalf("expected batch output %q, got %q", "redirected\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected no prompt on redirected stdin, got %q", got)
	}
}

func TestRun_forcesInteractiveMode_whenFlagProvidedAndStdinIsRedirected(t *testing.T) {
	// Given
	var stderr bytes.Buffer
	cmd := command{
		stdin:           bytes.NewBufferString("exit 0\n"),
		stdout:          &bytes.Buffer{},
		stderr:          &stderr,
		stdinIsTerminal: false,
	}

	// When
	err := cmd.run(context.Background(), []string{"nemosh", "-i"})

	// Then
	if err != nil {
		t.Fatalf("expected -i to force interactive mode, got %v", err)
	}
	if got := withoutANSI(stderr.String()); !strings.HasPrefix(got, "# ") || !strings.HasSuffix(got, "\n"+promptSymbol()+" ") {
		t.Fatalf("expected informative default prompt, got %q", got)
	}
}
