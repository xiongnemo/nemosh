package runtime_test

import (
	"bytes"
	"context"
	stdruntime "runtime"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	shellruntime "github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_unamePrintsSysname_whenRunWithoutOptions(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := shellruntime.New(applets.DefaultRegistry, shellruntime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "uname\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got, want := stdout.String(), expectedRuntimeUnameSysname()+"\n"; got != want {
		t.Fatalf("expected stdout %q, got %q", want, got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func expectedRuntimeUnameSysname() string {
	switch stdruntime.GOOS {
	case "windows":
		return "Windows_NT"
	case "darwin":
		return "Darwin"
	default:
		return "Linux"
	}
}
