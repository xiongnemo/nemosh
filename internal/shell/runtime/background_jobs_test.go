package runtime_test

import (
	"bytes"
	"context"
	"io"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

type backgroundApplet struct {
	name string
	run  func(context.Context, []string, io.Reader, io.Writer, io.Writer) error
}

func (a backgroundApplet) Name() string { return a.name }

func (a backgroundApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	return a.run(ctx, args, stdin, stdout, stderr)
}

func TestRuntime_backgroundLaunchContinuesImmediately_whenWorkerIsBlocked(t *testing.T) {
	// Given
	started := make(chan struct{})
	release := make(chan struct{})
	continued := make(chan struct{})
	registry := applets.NewRegistry(
		backgroundApplet{name: "block", run: func(ctx context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
			close(started)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}},
		backgroundApplet{name: "continued", run: func(_ context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
			close(continued)
			return nil
		}},
	)
	rt := runtime.New(registry, runtime.Streams{})
	result := make(chan int, 1)
	var releaseOnce sync.Once
	closeRelease := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(closeRelease)

	// When
	go func() { result <- rt.RunScript(context.Background(), "block & continued\n") }()

	// Then
	select {
	case <-continued:
	case <-time.After(2 * time.Second):
		t.Fatal("parent did not continue while background worker remained blocked")
	}
	select {
	case status := <-result:
		if status != 0 {
			t.Fatalf("parent status = %d, want 0", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parent did not return immediately after background launch")
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("background worker did not start")
	}
}

func TestRuntime_jobsObservesRunningAndWaitReturnsCachedStatus_whenWorkerCompletes(t *testing.T) {
	// Given
	started := make(chan struct{})
	release := make(chan struct{})
	registry := applets.NewRegistry(backgroundApplet{name: "block-false", run: func(ctx context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		close(started)
		select {
		case <-release:
			return applets.ErrExitFalse
		case <-ctx.Done():
			return ctx.Err()
		}
	}})
	var stdout, stderr bytes.Buffer
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	launch := make(chan int, 1)
	var releaseOnce sync.Once
	closeRelease := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(closeRelease)

	// When
	go func() { launch <- rt.RunScript(context.Background(), "block-false &\n") }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("background worker did not start")
	}
	jobsStatus := rt.RunScript(context.Background(), "jobs\n")
	closeRelease()
	select {
	case status := <-launch:
		if status != 0 {
			t.Fatalf("launch status = %d, want 0", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("background launch did not return")
	}
	waitStatus := rt.RunScript(context.Background(), "wait %1\n")

	// Then
	if jobsStatus != 0 || stdout.String() != "[1] Running\n" {
		t.Fatalf("jobs status = %d, stdout = %q", jobsStatus, stdout.String())
	}
	if waitStatus != 1 || stderr.String() != "" {
		t.Fatalf("wait status = %d, stderr = %q", waitStatus, stderr.String())
	}
}

// `$!` names the job that was just started.
//
// A **job specification** and not a process id, which is forced rather than chosen: a
// background job in this shell is a goroutine, so there is no pid to report. Naming the
// job keeps the two things `$!` is actually used for working, and a number here would have
// been a pid-shaped lie that `kill` would apply to some other process entirely.
func TestBackground_dollarBangNamesTheJob(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "it is set", script: "sleep 0.01 & echo \"[$!]\"\nwait\n", want: "[%1]\n"},
		{name: "it follows the newest", script: "sleep 0.01 &\nsleep 0.01 & echo \"[$!]\"\nwait\n", want: "[%2]\n"},
		// The two uses that have to keep working.
		{name: "kill takes it", script: "sleep 5 & kill $!\necho \"[$?]\"\n", want: "[0]\n"},
		{name: "wait takes it", script: "sleep 0.01 & wait $!\necho \"[$?]\"\n", want: "[0]\n"},
		{name: "jobs -p names it the same way", script: "sleep 0.5 & p=$!\n[ \"$(jobs -p)\" = \"$p\" ] && echo same\nwait\n", want: "same\n"},
		// Empty before any background job, which is what bash answers too -- measured,
		// because `${!-unset}` is the indirect-expansion syntax rather than a default
		// and so cannot be used to ask the question.
		{name: "empty before any job", script: "echo \"[$!]\"\n", want: "[]\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, stdout, stderr := runSetScript(t, test.script)
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			// A job that is a process names itself by its pid, as both references do; see
			// docs/design/background-processes.md.
			if runtime.JobsAreProcesses() && strings.HasPrefix(test.want, "[%") {
				if !regexp.MustCompile(`^\[[0-9]+\]\n$`).MatchString(stdout) {
					t.Fatalf("%s\n  got  %q\n  want a pid", test.script, stdout)
				}
				return
			}
			if stdout != test.want {
				t.Fatalf("%s\n  got  %q\n  want %q", test.script, stdout, test.want)
			}
		})
	}
}
