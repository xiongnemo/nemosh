package runtime_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_redirectsStderrToStdout_whenCommandUsesFdDuplication(t *testing.T) {
	// Given
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("expected test executable path, got %v", err)
	}
	t.Setenv("NEMOSH_REDIRECT_HELPER_PROCESS", "1")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), filepath.ToSlash(exe)+" -test.run=TestRedirectHelperProcess -- stderr-ok 2>&1\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "stderr-ok\n" {
		t.Fatalf("expected redirected stdout %q, got %q", "stderr-ok\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestRedirectHelperProcess(t *testing.T) {
	if os.Getenv("NEMOSH_REDIRECT_HELPER_PROCESS") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg != "--" {
			continue
		}
		fmt.Fprintln(os.Stderr, os.Args[i+1])
		os.Exit(0)
	}
	os.Exit(2)
}
