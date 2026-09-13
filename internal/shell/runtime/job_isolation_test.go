package runtime_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

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

// TestRuntime_outerJobBecomesDoneAfterNestedScopeDrains pins that `echo $(nested-block &) &`
// is reported Running until the job started inside its command substitution has gone.
//
// The nested applet must ignore cancellation, and that is the whole point rather than an
// oversight. commandSubstitutionScript ends with cancelAndDrain (expand_parameter.go), so a
// nested job that returns on ctx.Done() is gone the instant the substitution closes -- which
// left every step after this: the drain, echo's write, and the outer job finishing, racing
// the `jobs` below with no ordering between them. It passed only by winning that race. A
// 50ms pause after <-started turned it into "\n[1] Done\n" every time, and a loaded macOS
// runner caught it half way as "\n[1] Running\n".
//
// Blocking on release alone is what a command that does not die on cancel really does, and
// it makes the drain block, so the sequence below is ordered by channels rather than by
// timing: jobs runs, then release, then the nested applet returns, then the drain, then echo
// writes, then the job is done and `wait` can see it.
func TestRuntime_outerJobBecomesDoneAfterNestedScopeDrains(t *testing.T) {
	// Given
	started := make(chan struct{})
	release := make(chan struct{})
	registry := applets.NewRegistry(backgroundApplet{name: "nested-block", run: func(_ context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-time.After(time.Minute):
			// Only reachable if the runtime stopped letting the test get as far as
			// releasing it; a bounded wait reports that as a failure rather than as a
			// package-wide timeout, and unblocks this goroutine if an assertion above
			// left early.
			return errors.New("nested-block was never released")
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
	// Asserted before the release, because this is the property in the name: the nested job
	// is still there, so the outer job is Running and its echo has not run at all.
	if jobsStatus != 0 || stdout.String() != "[1] Running\n" {
		close(release)
		t.Fatalf("with the nested job still alive: jobs status = %d, stdout = %q", jobsStatus, stdout.String())
	}
	close(release)
	waitStatus := rt.RunScript(context.Background(), "wait %1\n")

	// Then
	if waitStatus != 0 || stdout.String() != "[1] Running\n\n" {
		t.Fatalf("wait status = %d, stdout = %q", waitStatus, stdout.String())
	}
}
