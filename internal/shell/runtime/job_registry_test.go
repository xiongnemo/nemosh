package runtime_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_waitAllConsumesCapturedJobs_andJobsIsObservational(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "false & true & jobs\nwait\njobs\n")

	// Then
	if status != 0 || !validJobLines(stdout.String(), 1, 2) {
		t.Fatalf("status = %d, stdout = %q", status, stdout.String())
	}
}

func TestRuntime_waitRejectsBadOperands_andRetainsExactJob(t *testing.T) {
	tests := []string{"wait 1", "wait %x", "wait %99", "wait %1 %2"}
	for _, script := range tests {
		t.Run(script, func(t *testing.T) {
			// Given
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

			// When
			status := rt.RunScript(context.Background(), script+"\n")

			// Then
			if status != 2 {
				t.Fatalf("status = %d, want 2", status)
			}
		})
	}
}

func TestRuntime_jobIDsAreMonotonic_andUnconsumedJobsAreCapped(t *testing.T) {
	// Given
	firstStarted := make(chan struct{}, 64)
	firstRelease := make(chan struct{})
	secondStarted := make(chan struct{})
	secondRelease := make(chan struct{})
	registry := applets.NewRegistry(
		backgroundApplet{name: "cap-first", run: func(ctx context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
			firstStarted <- struct{}{}
			select {
			case <-firstRelease:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}},
		backgroundApplet{name: "cap-second", run: func(ctx context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
			close(secondStarted)
			select {
			case <-secondRelease:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}},
	)
	var script strings.Builder
	for range 64 {
		script.WriteString("cap-first &\n")
	}
	script.WriteString("cap-first &\n")
	var stdout bytes.Buffer
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), script.String())
	for range 64 {
		<-firstStarted
	}
	jobsStatus := rt.RunScript(context.Background(), "jobs\n")
	close(firstRelease)
	waitStatus := rt.RunScript(context.Background(), "wait\n")
	launchStatus := rt.RunScript(context.Background(), "cap-second &\n")
	<-secondStarted
	secondJobsStatus := rt.RunScript(context.Background(), "jobs\n")
	close(secondRelease)
	lastWaitStatus := rt.RunScript(context.Background(), "wait %65\n")

	// Then
	if status != 1 || jobsStatus != 0 || waitStatus != 0 || launchStatus != 0 || secondJobsStatus != 0 || lastWaitStatus != 0 {
		t.Fatalf("statuses = %d, %d, %d, %d, %d, %d", status, jobsStatus, waitStatus, launchStatus, secondJobsStatus, lastWaitStatus)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 65 || lines[0] != "[1] Running" || lines[63] != "[64] Running" || lines[64] != "[65] Running" {
		t.Fatalf("unexpected jobs output: %q", stdout.String())
	}
}

func TestRuntime_jobLimitIsSessionWide_acrossRootAndPrivateScopes(t *testing.T) {
	// Given
	started := make(chan struct{}, 64)
	registry := applets.NewRegistry(backgroundApplet{
		name: "session-job",
		run: func(context.Context, []string, io.Reader, io.Writer, io.Writer) error {
			started <- struct{}{}
			return nil
		},
	})
	var script strings.Builder
	script.WriteString("{\n")
	for range 64 {
		script.WriteString("session-job &\n")
	}
	script.WriteString("} &\n")
	var stderr bytes.Buffer
	rt := runtime.New(registry, runtime.Streams{Stderr: &stderr})

	// When
	launchStatus := rt.RunScript(context.Background(), script.String())
	waitStatus := rt.RunScript(context.Background(), "wait %1\n")

	// Then
	if launchStatus != 0 || waitStatus != 1 || len(started) != 63 {
		t.Fatalf("statuses = %d, %d, nested starts = %d; want 0, 1, 63", launchStatus, waitStatus, len(started))
	}
	if got := stderr.String(); got != "nemosh: job limit reached\n" {
		t.Fatalf("stderr = %q, want job-limit diagnostic", got)
	}
}

func validJobLines(output string, ids ...int) bool {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != len(ids) {
		return false
	}
	for index, id := range ids {
		prefix := fmt.Sprintf("[%d] ", id)
		if !strings.HasPrefix(lines[index], prefix) || (lines[index] != prefix+"Running" && lines[index] != prefix+"Done" && !strings.HasPrefix(lines[index], prefix+"Done(")) {
			return false
		}
	}
	return true
}

func TestRuntime_waitTargetReturnsCachedPipelineStatus_withPipefail(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   int
	}{
		{name: "last stage", script: "false | true & wait %1\n", want: 0},
		{name: "pipefail", script: "set -o pipefail\nfalse | true & wait %1\n", want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

			// When
			status := rt.RunScript(context.Background(), test.script)

			// Then
			if status != test.want {
				t.Fatalf("status = %d, want %d for %s", status, test.want, fmt.Sprintf("%q", test.script))
			}
		})
	}
}
