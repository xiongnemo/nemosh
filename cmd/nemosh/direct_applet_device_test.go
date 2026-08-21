//go:build windows

// The device model is Windows-only, so these are too.
//
// They were cross-platform, and passed everywhere, because the shell used to invent a /dev on every
// platform -- and for null, zero and random an invented device behaves like the real one, so nothing
// looked wrong. It was wrong: on a system with its own /dev the invention shadows it. Now that only
// Windows gets one, these assertions describe Windows.
//
// Two of them show why the old arrangement could not stay. `cat /dev` reported "unsupported device"
// where macOS answers "Is a directory", the real answer for a real directory. And `cat /dev/stdin`
// returned this harness's injected buffer, where on a platform with a real /dev it reads the
// process's actual descriptor 0 -- which is what every other program there does.

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestP05WaveA_directCatAndWc_readDeviceOperands_inBothEntryForms(t *testing.T) {
	tests := []struct {
		name, stdin, want string
		args              []string
	}{
		{name: "explicit cat stdin", args: []string{"nemosh", "cat", "/dev/stdin"}, stdin: "bounded\n", want: "bounded\n"},
		{name: "multicall cat stdin", args: []string{directAppletInvocationName("cat"), "/dev/stdin"}, stdin: "bounded\n", want: "bounded\n"},
		{name: "explicit wc null", args: []string{"nemosh", "wc", "/dev/null"}, want: "        0         0         0 /dev/null\n"},
		{name: "multicall wc null", args: []string{directAppletInvocationName("wc"), "/dev/null"}, want: "        0         0         0 /dev/null\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			stdin := &directCloseTrackingReader{Buffer: bytes.NewBufferString(test.stdin)}
			var stdout, stderr bytes.Buffer
			cmd := command{stdin: stdin, stdout: &stdout, stderr: &stderr}

			// When
			err := cmd.run(context.Background(), test.args)

			// Then
			if err != nil || stdout.String() != test.want || stderr.Len() != 0 {
				t.Fatalf("run(%v): stdout=%q stderr=%q error=%v", test.args, stdout.String(), stderr.String(), err)
			}
			if stdin.closed {
				t.Fatal("direct applet closed caller-owned stdin")
			}
		})
	}
}

func TestP05WaveA_directCat_rejectsExactDev_inBothEntryForms(t *testing.T) {
	for _, args := range [][]string{
		{"nemosh", "cat", "/dev"},
		{directAppletInvocationName("cat"), "/dev"},
	} {
		// Given
		var stdout, stderr bytes.Buffer
		cmd := command{stdin: bytes.NewReader(nil), stdout: &stdout, stderr: &stderr}

		// When
		err := cmd.run(context.Background(), args)

		// Then
		if err == nil || !strings.Contains(err.Error(), "/dev: unsupported device") {
			t.Fatalf("run(%v): error=%v", args, err)
		}
		if stdout.Len() != 0 {
			t.Fatalf("run(%v): stdout=%q, want nothing", args, stdout.String())
		}
		// Reported under the applet's name, the same way the shell reports it.
		// This used to be silent, which is what made a direct invocation fail
		// differently from the same command inside the shell.
		if !strings.Contains(stderr.String(), "cat: ") || !strings.Contains(stderr.String(), "unsupported device") {
			t.Fatalf("run(%v): stderr=%q, want a cat-prefixed device diagnostic", args, stderr.String())
		}
	}
}

type directCloseTrackingReader struct {
	*bytes.Buffer
	closed bool
}

func (r *directCloseTrackingReader) Close() error {
	r.closed = true
	return nil
}
