package runtime

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestFinalSecurity_enabledExactDevRedirectsFailBeforeHostEffects(t *testing.T) {
	tests := []struct {
		name      string
		operation redirectOperation
	}{
		{name: "input", operation: redirectOperation{kind: redirectInput, target: 0, path: "/dev"}},
		{name: "output", operation: redirectOperation{kind: redirectOutput, target: 1, path: "/dev"}},
		{name: "append", operation: redirectOperation{kind: redirectAppend, target: 1, path: "/dev"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			runtime := New(applets.DefaultRegistry, Streams{})
			table := newFDTable(Streams{})

			// When
			err := runtime.applyRedirectOperations(table, []redirectOperation{test.operation})

			// Then
			if !errors.Is(err, errUnsupportedDevice) {
				t.Fatalf("exact /dev %s redirect error: got %v want %v", test.name, err, errUnsupportedDevice)
			}
		})
	}
}

func TestFinalSecurity_enabledDeviceRedirectsPreserveTypedDiagnostics(t *testing.T) {
	tests := []struct {
		name      string
		operation redirectOperation
		want      error
	}{
		{name: "unknown input", operation: redirectOperation{kind: redirectInput, target: 0, path: "/dev/not-a-device"}, want: errUnsupportedDevice},
		{name: "unknown output", operation: redirectOperation{kind: redirectOutput, target: 1, path: "/dev/not-a-device"}, want: errUnsupportedDevice},
		{name: "malformed input fd", operation: redirectOperation{kind: redirectInput, target: 0, path: "/dev/fd/x"}, want: errMalformedDeviceFD},
		{name: "malformed output fd", operation: redirectOperation{kind: redirectOutput, target: 1, path: "/dev/fd/x"}, want: errMalformedDeviceFD},
		{name: "unreadable stdout", operation: redirectOperation{kind: redirectInput, target: 0, path: "/dev/stdout"}, want: errDescriptorNotReadable},
		{name: "unwritable stdin", operation: redirectOperation{kind: redirectOutput, target: 1, path: "/dev/stdin"}, want: errDescriptorNotWritable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			runtime := New(applets.DefaultRegistry, Streams{})
			table := newFDTable(Streams{})

			// When
			err := runtime.applyRedirectOperations(table, []redirectOperation{test.operation})

			// Then
			if !errors.Is(err, test.want) {
				t.Fatalf("%s error: got %v want %v", test.name, err, test.want)
			}
		})
	}
}

func TestFinalSecurity_hostAndSupportedDeviceRedirectsRemainUnchanged(t *testing.T) {
	// Given
	directory := t.TempDir()
	inputPath := filepath.Join(directory, "input.txt")
	outputPath := filepath.Join(directory, "output.txt")
	if err := os.WriteFile(inputPath, []byte("host\n"), 0o600); err != nil {
		t.Fatalf("write input fixture: %v", err)
	}
	var stdout bytes.Buffer
	runtime := NewWithState(applets.DefaultRegistry, Streams{Stdout: &stdout}, State{Cwd: WorkingDirectory(directory)})

	// When
	status := runtime.RunScript(t.Context(), "cat < input.txt > output.txt\necho visible > /dev/stdout\ncat /dev/null\n")

	// Then
	if status != 0 {
		t.Fatalf("redirect status: %d", status)
	}
	contents, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output fixture: %v", err)
	}
	if got, want := string(contents), "host\n"; got != want {
		t.Fatalf("host redirect contents: got %q want %q", got, want)
	}
	if got, want := stdout.String(), "visible\n"; got != want {
		t.Fatalf("device redirect output: got %q want %q", got, want)
	}
}

func TestFinalSecurity_appendRedirectPreservesExistingContents(t *testing.T) {
	// Given
	directory := t.TempDir()
	path := filepath.Join(directory, "append.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o600); err != nil {
		t.Fatalf("write append fixture: %v", err)
	}
	runtime := NewWithState(applets.DefaultRegistry, Streams{}, State{Cwd: WorkingDirectory(directory)})

	// When
	status := runtime.RunScript(t.Context(), "echo after >> append.txt\n")

	// Then
	if status != 0 {
		t.Fatalf("append redirect status: %d", status)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read append fixture: %v", err)
	}
	if got, want := string(contents), "before\nafter\n"; got != want {
		t.Fatalf("append contents: got %q want %q", got, want)
	}
}

func TestFinalSecurity_deviceTraversalRedirectRemainsHostBehavior(t *testing.T) {
	// Given
	directory := t.TempDir()
	spacedDirectory := filepath.Join(directory, "nested path")
	if err := os.Mkdir(spacedDirectory, 0o700); err != nil {
		t.Fatalf("create spaced traversal directory: %v", err)
	}
	inputPath := filepath.Join(spacedDirectory, "input.txt")
	outputPath := filepath.Join(spacedDirectory, "output.txt")
	if err := os.WriteFile(inputPath, []byte("escaped\n"), 0o600); err != nil {
		t.Fatalf("write traversal fixture: %v", err)
	}
	volume := filepath.ToSlash(filepath.VolumeName(directory))
	inputOperand := "/dev/.." + strings.TrimPrefix(filepath.ToSlash(inputPath), volume)
	outputOperand := "/dev/.." + strings.TrimPrefix(filepath.ToSlash(outputPath), volume)
	runtime := NewWithState(applets.DefaultRegistry, Streams{}, State{Cwd: WorkingDirectory(directory)})

	// When
	status := runtime.RunScript(t.Context(), "cat < '"+inputOperand+"' > '"+outputOperand+"'\n")

	// Then
	if status != 0 {
		t.Fatalf("traversal redirect status: %d", status)
	}
	contents, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read traversal output: %v", err)
	}
	if got, want := string(contents), "escaped\n"; got != want {
		t.Fatalf("traversal contents: got %q want %q", got, want)
	}
}
