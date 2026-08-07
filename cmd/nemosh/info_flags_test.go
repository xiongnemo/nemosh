package main

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func runInfoFlag(t *testing.T, argument string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := command{stdin: strings.NewReader(""), stdout: &stdout, stderr: &stderr}
	err := cmd.run(context.Background(), []string{"nemosh", argument})
	return stdout.String(), stderr.String(), err
}

// The version has to be readable without entering a terminal, because it is
// what a bug report quotes and what a package manager compares.
func TestVersionFlag_printsOneLineAndExitsZero(t *testing.T) {
	// When
	stdout, stderr, err := runInfoFlag(t, "--version")

	// Then
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %q", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout = %q, want exactly one line", stdout)
	}
	if !strings.HasPrefix(lines[0], "nemosh v") {
		t.Fatalf("stdout = %q, want it to start with %q", stdout, "nemosh v")
	}
	// The runtime and platform belong on the line, so a report identifies the
	// build rather than just the tag.
	for _, want := range []string{"go1.", "/"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("stdout = %q, want it to contain %q", stdout, want)
		}
	}
}

// `--list` generates Scoop shims, so the format is deliberately plain: one
// applet per line, sorted, nothing else on the line.
func TestListFlag_printsEveryAppletOnePerLine(t *testing.T) {
	// When
	stdout, stderr, err := runInfoFlag(t, "--list")

	// Then
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %q", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	got := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	want := applets.DefaultRegistry.Names()
	if !slices.Equal(got, want) {
		t.Fatalf("--list printed %d names, registry has %d\n  got:  %v\n  want: %v", len(got), len(want), got, want)
	}
	for _, line := range got {
		if strings.TrimSpace(line) != line || line == "" {
			t.Errorf("line %q carries decoration or padding", line)
		}
	}
}

// An applet must still win over the flag spelling, and neither flag may be
// mistaken for the "invalid option" path that used to swallow both.
func TestInfoFlags_doNotDisturbOrdinaryDispatch(t *testing.T) {
	for _, test := range []struct {
		name     string
		argument string
		wantErr  bool
	}{
		{name: "an unknown long option still fails", argument: "--nosuchflag", wantErr: true},
		{name: "version is not an unknown option", argument: "--version"},
		{name: "list is not an unknown option", argument: "--list"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, stderr, err := runInfoFlag(t, test.argument)

			// Then
			if test.wantErr {
				if err == nil {
					t.Fatal("run() error = nil, want an invalid-option failure")
				}
				if !strings.Contains(stderr, "invalid option") {
					t.Fatalf("stderr = %q, want an invalid-option diagnostic", stderr)
				}
				return
			}
			if err != nil {
				t.Fatalf("run() error = %v", err)
			}
		})
	}
}
