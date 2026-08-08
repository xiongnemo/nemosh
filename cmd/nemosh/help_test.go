package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func runHelp(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := command{stdin: strings.NewReader(""), stdout: &stdout, stderr: &stderr}
	err := cmd.run(context.Background(), append([]string{"nemosh"}, args...))
	return stdout.String(), stderr.String(), err
}

// Help goes to stdout and exits 0, because it was asked for. A diagnostic goes
// to stderr with a non-zero status; conflating the two makes `nemosh --help |
// less` print nothing.
func TestHelpFlag_writesToStdoutAndSucceeds(t *testing.T) {
	// When
	stdout, stderr, err := runHelp(t, "--help")

	// Then
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %q", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if stdout == "" {
		t.Fatal("stdout is empty")
	}
}

// Every invocation form and option the binary really has must appear, so the
// help cannot drift into describing a shell this is not.
func TestHelpFlag_namesEveryInvocationFormAndOption(t *testing.T) {
	// When
	stdout, _, err := runHelp(t, "--help")
	if err != nil {
		t.Fatal(err)
	}

	// Then
	for _, want := range []string{
		"Usage:",
		"-c ",       // run a command string
		"-i",        // force interactive
		"--version", // the two info flags
		"--list",
		"--help",
		"NEMOSH_DEBUG",            // the documented environment channel
		"NEMOSH_OVERRIDE_APPLETS", // the documented override
		"multicall",               // the property that explains applet dispatch
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help does not mention %q", want)
		}
	}
}

// The applet count is read from the registry rather than typed into the text,
// so adding an applet cannot leave the help quietly wrong.
func TestHelpFlag_reportsTheRealAppletCount(t *testing.T) {
	// Given
	registered := len(applets.DefaultRegistry.Names())

	// When
	stdout, _, err := runHelp(t, "--help")
	if err != nil {
		t.Fatal(err)
	}

	// Then
	if !strings.Contains(stdout, "--list") {
		t.Fatal("help does not point at --list")
	}
	if registered == 0 {
		t.Fatal("registry is empty")
	}
	if !strings.Contains(stdout, itoa(registered)) {
		t.Fatalf("help does not report the real applet count %d", registered)
	}
}

// Applets carry no usage text, which is a divergence from busybox rather than
// an omission. The help says so instead of leaving the reader to discover it
// from `nemosh cat --help`.
func TestHelpFlag_saysAppletsCarryNoUsageText(t *testing.T) {
	// When
	stdout, _, err := runHelp(t, "--help")
	if err != nil {
		t.Fatal(err)
	}

	// Then
	if !strings.Contains(stdout, "support-matrix") {
		t.Error("help does not point at the support matrix")
	}
	if !strings.Contains(strings.ToLower(stdout), "usage text") {
		t.Error("help does not say applets carry no usage text")
	}
}

// An unusable option should say where the usable ones are listed.
func TestInvalidOption_pointsAtHelp(t *testing.T) {
	// When
	stdout, stderr, err := runHelp(t, "--nosuchflag")

	// Then
	if err == nil {
		t.Fatal("run() error = nil, want a refusal")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want the diagnostic on stderr only", stdout)
	}
	if !strings.Contains(stderr, "invalid option") {
		t.Fatalf("stderr = %q, want an invalid-option diagnostic", stderr)
	}
	if !strings.Contains(stderr, "--help") {
		t.Fatalf("stderr = %q, want it to point at --help", stderr)
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
