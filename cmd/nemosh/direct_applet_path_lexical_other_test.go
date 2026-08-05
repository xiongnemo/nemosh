//go:build !windows

package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	shellruntime "github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestDirectApplet_extraDotInvocationDoesNotSelectKnownAppletOnNonWindows(t *testing.T) {
	for _, argv0 := range []string{"pwd.extra", "pwd.extra.bin", "cat.tool"} {
		// Given
		var stdout, stderr bytes.Buffer
		cmd := command{stdin: &bytes.Buffer{}, stdout: &stdout, stderr: &stderr}

		// When
		err := cmd.run(context.Background(), []string{argv0})

		// Then
		if err != nil || stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatalf("run(%q): stdout=%q stderr=%q error=%v", argv0, stdout.String(), stderr.String(), err)
		}
	}

	stdout, stderr, err := runDirectAppletTest([]string{"pwd"})
	if err != nil || stdout == "" || stderr != "" {
		t.Fatalf("exact pwd: stdout=%q stderr=%q error=%v", stdout, stderr, err)
	}
}

func TestDirectPosixpath_preservesOrdinaryUnixPaths(t *testing.T) {
	// Given
	state := shellruntime.State{Cwd: "/workspace/project", Env: shellruntime.NewEnvironment(os.Environ())}
	tests := []struct {
		input string
		want  string
	}{
		{input: "/var/lib/nonexistent", want: "/var/lib/nonexistent\n"},
		{input: "relative/nonexistent", want: "/workspace/project/relative/nonexistent\n"},
	}
	for _, test := range tests {
		// When
		stdout, stderr, err := runDirectAppletStateTest([]string{"nemosh", "posixpath", test.input}, state)

		// Then
		if err != nil || stdout != test.want || stderr != "" {
			t.Fatalf("posixpath %q: stdout=%q stderr=%q error=%v, want %q", test.input, stdout, stderr, err, test.want)
		}
	}
}
