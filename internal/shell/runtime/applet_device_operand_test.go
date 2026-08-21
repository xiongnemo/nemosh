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

package runtime_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestP05WaveA_catAndWc_readRuntimeDeviceOperands(t *testing.T) {
	tests := []struct {
		name   string
		script string
		stdin  string
		want   string
	}{
		{name: "cat stdin", script: "cat /dev/stdin\n", stdin: "alpha\nbeta\n", want: "alpha\nbeta\n"},
		{name: "wc stdin", script: "wc /dev/stdin\n", stdin: "alpha\nbeta\n", want: "        2         2        11 /dev/stdin\n"},
		{name: "cat null", script: "cat /dev/null\n", want: ""},
		{name: "wc null", script: "wc /dev/null\n", want: "        0         0         0 /dev/null\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			var stdout, stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{
				Stdin: strings.NewReader(test.stdin), Stdout: &stdout, Stderr: &stderr,
			})

			// When
			status := rt.RunScript(context.Background(), test.script)

			// Then
			if status != 0 || stdout.String() != test.want || stderr.Len() != 0 {
				t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
			}
		})
	}
}

func TestP05WaveA_deviceOperandErrors_precedeHostFilesystemAccess(t *testing.T) {
	tests := []struct {
		name, path, want string
	}{
		{name: "exact root", path: "/dev", want: "unsupported device"},
		{name: "unknown", path: "/dev/not-a-device", want: "unsupported device"},
		{name: "malformed fd", path: "/dev/fd/x", want: "malformed /dev/fd descriptor"},
		{name: "write only", path: "/dev/stdout", want: "file descriptor is not readable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			var stdout, stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

			// When
			status := rt.RunScript(context.Background(), "cat "+test.path+"\n")

			// Then
			if status == 0 || stdout.Len() != 0 {
				t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
			}
			if got := stderr.String(); !strings.Contains(got, test.want) || strings.Contains(got, "host path") {
				t.Fatalf("stderr=%q want typed diagnostic containing %q", got, test.want)
			}
		})
	}
}

func TestP05WaveA_deviceTraversal_normalizesBeforeClassificationAndReadsHostBacking(t *testing.T) {
	// Given
	directory := t.TempDir()
	native := filepath.Join(directory, "escape.txt")
	if err := os.WriteFile(native, []byte("escaped-host\n"), 0o600); err != nil {
		t.Fatalf("write traversal fixture: %v", err)
	}
	volume := filepath.ToSlash(filepath.VolumeName(native))
	rootRelative := strings.TrimPrefix(filepath.ToSlash(native), volume)
	operand := "/dev/.." + rootRelative
	var stdout, stderr bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{
		Stdout: &stdout, Stderr: &stderr,
	}, runtime.State{Cwd: runtime.WorkingDirectory(directory)})

	// When
	status := rt.RunScript(context.Background(), "cat "+operand+"\n")

	// Then
	if status != 0 || stdout.String() != "escaped-host\n" || stderr.Len() != 0 {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
}

func TestP05WaveA_deviceCapability_survivesEnvAndXargs(t *testing.T) {
	tests := []struct {
		name, script, stdin, want string
	}{
		{name: "env cat", script: "env CHILD=value cat /dev/stdin\n", stdin: "nested\n", want: "nested\n"},
		{name: "env wc", script: "env -i wc /dev/null\n", want: "        0         0         0 /dev/null\n"},
		{name: "env xargs cat", script: "env CHILD=value xargs cat\n", stdin: "/dev/null\n", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			var stdout, stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{
				Stdin: strings.NewReader(test.stdin), Stdout: &stdout, Stderr: &stderr,
			})

			// When
			status := rt.RunScript(context.Background(), test.script)

			// Then
			if status != 0 || stdout.String() != test.want || stderr.Len() != 0 {
				t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
			}
		})
	}
}

func TestP05WaveA_mixedAndRepeatedDeviceOperands_preserveOrderLabelsAndOwnership(t *testing.T) {
	// Given
	directory := t.TempDir()
	path := filepath.Join(directory, "input.txt")
	if err := os.WriteFile(path, []byte("host\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	stdin := &closeTrackingReader{Reader: strings.NewReader("device\n")}
	var stdout, stderr bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{
		Stdin: stdin, Stdout: &stdout, Stderr: &stderr,
	}, runtime.State{Cwd: runtime.WorkingDirectory(directory)})
	script := "cat input.txt /dev/null /dev/stdin\nwc -l input.txt /dev/null /dev/null\n"

	// When
	status := rt.RunScript(context.Background(), script)

	// Then
	// Three operands, so wc ends with a total, which both references print and this
	// shell did not until the padding work. `-l` is one count, so it stays unpadded.
	want := "host\ndevice\n1 input.txt\n0 /dev/null\n0 /dev/null\n1 total\n"
	if status != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if stdin.closed {
		t.Fatal("caller-owned stdin was closed")
	}
}

func TestP05WaveA_allSharedStreamingApplets_acceptReadableNullOperand(t *testing.T) {
	// Given
	scripts := []string{
		"head /dev/null", "tail /dev/null", "grep match /dev/null",
		"cut -b 1 /dev/null", "sort /dev/null", "uniq /dev/null",
	}

	for _, script := range scripts {
		t.Run(script, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

			// When
			status := rt.RunScript(context.Background(), script+"\n")

			// Then
			wantStatus := 0
			if strings.HasPrefix(script, "grep ") {
				wantStatus = 1
			}
			if status != wantStatus || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
			}
		})
	}
}

type closeTrackingReader struct {
	io.Reader
	closed bool
}

func (r *closeTrackingReader) Close() error {
	r.closed = true
	return nil
}
