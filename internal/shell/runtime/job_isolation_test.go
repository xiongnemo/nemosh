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

func TestRuntime_backgroundStateControlAndTrapAreIsolated(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	parentCwd := t.TempDir()
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{Cwd: runtime.WorkingDirectory(parentCwd)})
	if status := rt.RunScript(context.Background(), "pwd\n"); status != 0 {
		t.Fatalf("baseline pwd status = %d", status)
	}
	baselinePwd := stdout.String()
	stdout.Reset()

	// When
	status := rt.RunScript(context.Background(), "value=parent\n{ value=child\ncd /\ntrap 'echo leaked' EXIT\nexit 7\n} &\nwait %1\necho $value\npwd\n")

	// Then
	wantOutput := "parent\n" + baselinePwd
	if status != 0 || stdout.String() != wantOutput || rt.WorkingDirectory() != displayPath(parentCwd) {
		t.Fatalf("status = %d, stdout = %q", status, stdout.String())
	}
}

func TestRuntime_backgroundStdinDefaultsToNullUnlessRedirected(t *testing.T) {
	// Given
	reads := make(chan string, 2)
	registry := applets.NewRegistry(backgroundApplet{name: "capture-stdin", run: func(_ context.Context, _ []string, stdin io.Reader, _ io.Writer, _ io.Writer) error {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return err
		}
		reads <- string(data)
		return nil
	}})
	parentStdin := bytes.NewBufferString("parent\n")
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.WriteFile(input, []byte("redirected\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rt := runtime.NewWithState(registry, runtime.Streams{Stdin: parentStdin}, runtime.State{Cwd: runtime.WorkingDirectory(dir)})

	// When
	status := rt.RunScript(context.Background(), "capture-stdin & wait %1\ncapture-stdin < input & wait %2\n")
	nullRead := <-reads
	redirectedRead := <-reads

	// Then
	if status != 0 || nullRead != "" || redirectedRead != "redirected\n" || parentStdin.String() != "parent\n" {
		t.Fatalf("status = %d, reads = %q/%q, parent stdin = %q", status, nullRead, redirectedRead, parentStdin.String())
	}
}

func TestRuntime_outerJobBecomesDoneAfterNestedScopeDrains(t *testing.T) {
	// Given
	started := make(chan struct{})
	release := make(chan struct{})
	registry := applets.NewRegistry(backgroundApplet{name: "nested-block", run: func(ctx context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}})
	var stdout bytes.Buffer
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout})

	// When
	if status := rt.RunScript(context.Background(), "echo $(nested-block &) &\n"); status != 0 {
		t.Fatalf("launch status = %d", status)
	}
	<-started
	jobsStatus := rt.RunScript(context.Background(), "jobs\n")
	close(release)
	waitStatus := rt.RunScript(context.Background(), "wait %1\n")

	// Then
	if jobsStatus != 0 || waitStatus != 0 || stdout.String() != "[1] Running\n\n" {
		t.Fatalf("statuses = %d, %d, stdout = %q", jobsStatus, waitStatus, stdout.String())
	}
}
