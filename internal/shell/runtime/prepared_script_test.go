package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestRuntime_executePrepared_runsParsedScript(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})
	script, err := ParseScript("echo prepared # removed\n")
	if err != nil {
		t.Fatalf("ParseScript() error = %v", err)
	}

	// When
	status, control := rt.executePrepared(context.Background(), script)

	// Then
	if status != 0 || control != flowNone {
		t.Fatalf("executePrepared() = (%d, %v), want (0, flowNone)", status, control)
	}
	if got := stdout.String(); got != "prepared\n" {
		t.Fatalf("executePrepared() stdout = %q, want %q", got, "prepared\n")
	}
}

func TestRuntime_RunScript_executesLogicalLinesFromParser(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{name: "multiline quote", source: "printf '%s' \"one\ntwo\"\n", want: "one\ntwo"},
		{name: "backslash continuation", source: "echo one\\\ntwo\n", want: "onetwo\n"},
		{name: "comments", source: "echo one # removed\necho '# kept'\n", want: "one\n# kept\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			var stdout bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

			// When
			status := rt.RunScript(context.Background(), tt.source)

			// Then
			if status != 0 {
				t.Fatalf("RunScript() status = %d, want 0", status)
			}
			if got := stdout.String(); got != tt.want {
				t.Fatalf("RunScript() stdout = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuntime_RunScript_reportsParserDiagnosticOnce(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "malformed", source: "fi\n"},
		{name: "incomplete", source: "echo 'open"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			var stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stderr: &stderr})

			// When
			status := rt.RunScript(context.Background(), tt.source)

			// Then
			if status != 2 {
				t.Fatalf("RunScript() status = %d, want 2", status)
			}
			if got := strings.Count(stderr.String(), "nemosh:"); got != 1 {
				t.Fatalf("RunScript() diagnostics = %d in %q, want 1", got, stderr.String())
			}
		})
	}
}

func TestRuntime_RunScript_doesNotExecuteCompletePrefixBeforeLaterSyntaxError(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	source := "echo before\nif true\nthen\necho compound\nfi\nfi\n"

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	if status != 2 {
		t.Fatalf("RunScript() status = %d, want 2", status)
	}
	if got, want := stdout.String(), ""; got != want {
		t.Fatalf("RunScript() stdout = %q, want %q", got, want)
	}
	if got := strings.Count(stderr.String(), "nemosh:"); got != 1 {
		t.Fatalf("RunScript() diagnostics = %d in %q, want 1", got, stderr.String())
	}
}

func TestRuntime_recursiveScriptEntryPointsUsePreparedExecution(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dir := t.TempDir()
	sourcePath := filepath.ToSlash(filepath.Join(dir, "source.sh"))
	if err := os.WriteFile(sourcePath, []byte("echo sourced # removed\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	source := fmt.Sprintf("trap 'echo trapped # removed' EXIT\neval echo evaluated # removed\n. %s\necho $(echo substituted # removed\n)\n", sourcePath)

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	if status != 0 {
		t.Fatalf("RunScript() status = %d, want 0; stderr = %q", status, stderr.String())
	}
	if got, want := stdout.String(), "evaluated\nsourced\nsubstituted\ntrapped\n"; got != want {
		t.Fatalf("RunScript() stdout = %q, want %q", got, want)
	}
}
