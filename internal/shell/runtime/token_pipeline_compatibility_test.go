package runtime_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_tokenPipelineRoutesExternalOutputToApplet(t *testing.T) {
	// Given
	executable := pipelineHelperExecutable(t)
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), executable+" -test.run=TestRuntimeHelperProcess -- external-line | head -n 1\n")

	// Then
	if status != 0 || stdout.String() != "external-line\n" {
		t.Fatalf("status=%d stdout=%q", status, stdout.String())
	}
}

func TestRuntime_tokenPipelineRoutesAppletOutputToExternal(t *testing.T) {
	// Given
	executable := pipelineHelperExecutable(t)
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "echo applet-line | "+executable+" -test.run=TestRuntimeHelperProcess -- copy-stdin\n")

	// Then
	if status != 0 || stdout.String() != "applet-line\n" {
		t.Fatalf("status=%d stdout=%q", status, stdout.String())
	}
}

func TestRuntime_nativeProducerTreatsEarlyDownstreamClosureAsSuccessByDefault(t *testing.T) {
	// Given
	executable := pipelineHelperExecutable(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	registry := applets.NewRegistry(pipelineApplet{name: "read-one", run: readOneByte})
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), executable+" -test.run=TestRuntimeHelperProcess -- write-large | read-one\n")

	// Then
	if status != 0 || stdout.String() != "x" || stderr.Len() != 0 {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
}

func TestRuntime_nativeProducerEarlyDownstreamClosureHonorsPipefail(t *testing.T) {
	// Given
	executable := pipelineHelperExecutable(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	registry := applets.NewRegistry(pipelineApplet{name: "read-one", run: readOneByte})
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "set -o pipefail\n"+executable+" -test.run=TestRuntimeHelperProcess -- write-large | read-one\n")

	// Then
	if status != 0 || stdout.String() != "x" || stderr.Len() != 0 {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
}

func readOneByte(_ context.Context, _ []string, stdin io.Reader, stdout, _ io.Writer) error {
	_, err := io.CopyN(stdout, stdin, 1)
	return err
}

func pipelineHelperExecutable(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("test executable: %v", err)
	}
	t.Setenv("NEMOSH_RUNTIME_HELPER_PROCESS", "1")
	return filepath.ToSlash(executable)
}
