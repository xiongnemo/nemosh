package runtime

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func runKill(t *testing.T, script string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{
		Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr,
	})
	status := rt.RunScript(context.Background(), script)
	return stdout.String(), stderr.String(), status
}

// `kill %N` is the reason kill is a builtin rather than an applet: only the shell
// has the job table. busybox's killcmd exists for exactly that and does nothing
// else -- it translates %N into the job's pids and hands them to the ordinary
// kill (shell/ash.c:4787-4830).
//
// Here there is nothing to translate into, because a background job is a
// goroutine and has no pid. What it has is its own context, so the signal arrives
// as a cancellation. `jobs` reporting Done afterwards is the observable half.
func TestKill_stopsABackgroundJob(t *testing.T) {
	// Given / When: a job that would run for half a minute, killed at once
	stdout, stderr, status := runKill(t, "sleep 30 &\nkill %1\nwait\njobs\n")

	// Then
	if stderr != "" {
		t.Fatalf("stderr = %q, want nothing", stderr)
	}
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	// `wait` returned, which it could not have done in under thirty seconds
	// unless the job actually stopped.
	if strings.Contains(stdout, "Running") {
		t.Fatalf("jobs still reports it running:\n%s", stdout)
	}
}

// And it does not take ownership of the job, so a later `wait` still works. That
// is why kill looks the record up rather than claiming it.
func TestKill_leavesTheJobWaitable(t *testing.T) {
	// Given / When
	_, stderr, status := runKill(t, "sleep 30 &\nkill %1\nwait %1\n")

	// Then: the job is still there to be waited for. Its status is deliberately
	// not asserted to be zero -- a killed job did not succeed, and `wait`
	// reporting that is the point of `wait`. What must not happen is `wait`
	// failing to find a job that kill only signalled.
	if stderr != "" {
		t.Fatalf("wait complained after kill: %s", stderr)
	}
	if status == 2 {
		t.Fatal("wait reported an unknown or busy job, so kill had claimed it")
	}
}

func TestKill_refusesWhatItCannotDo(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
		status int
	}{
		{name: "no such job", script: "kill %9\n", want: "no such job", status: 1},
		{name: "not a job and not a number", script: "kill nope\n", want: "illegal pid: nope", status: 1},
		{name: "job zero", script: "kill %0\n", want: "invalid job", status: 1},
		{name: "an unknown signal", script: "kill -BOGUS 1\n", want: "invalid signal", status: 2},
		{name: "nothing at all", script: "kill\n", want: "expected a job or a process id", status: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, stderr, status := runKill(t, test.script)

			// Then
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want it to mention %q", stderr, test.want)
			}
			if status != test.status {
				t.Fatalf("status = %d, want %d", status, test.status)
			}
		})
	}
}

// Both spellings of a signal, because a script writes the number and a person
// writes the name, and refusing either refuses half the users.
func TestKill_acceptsASignalEitherWay(t *testing.T) {
	for _, script := range []string{
		"sleep 30 &\nkill -9 %1\nwait\n",
		"sleep 30 &\nkill -TERM %1\nwait\n",
		"sleep 30 &\nkill -SIGTERM %1\nwait\n",
		"sleep 30 &\nkill -15 %1\nwait\n",
	} {
		t.Run(script, func(t *testing.T) {
			_, stderr, status := runKill(t, script)
			if stderr != "" || status != 0 {
				t.Fatalf("stderr = %q, status = %d", stderr, status)
			}
		})
	}
}

func TestKill_listsSignals(t *testing.T) {
	// When
	stdout, _, status := runKill(t, "kill -l\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d", status)
	}
	for _, want := range []string{"SIGHUP", "SIGKILL", "SIGTERM"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("kill -l did not list %s:\n%s", want, stdout)
		}
	}
}

// A pid operand kills a real process, which is the half that has nothing to do
// with jobs. Tested against a process this test starts, so nothing else on the
// machine is at risk.
func TestKill_terminatesARealProcess(t *testing.T) {
	// Given: a child that would outlive the test
	child := exec.Command(sleepingHelper(t), "-c", "sleep 60")
	if err := child.Start(); err != nil {
		t.Fatalf("starting the helper: %v", err)
	}
	defer func() { _ = child.Process.Kill() }()
	pid := child.Process.Pid

	// When
	_, stderr, status := runKill(t, "kill "+strconv.Itoa(pid)+"\n")

	// Then
	if stderr != "" || status != 0 {
		t.Fatalf("kill %d: stderr = %q, status = %d", pid, stderr, status)
	}
	waited := make(chan error, 1)
	go func() { waited <- child.Wait() }()
	select {
	case <-waited:
	case <-time.After(20 * time.Second):
		t.Fatalf("process %d survived being killed", pid)
	}

	// And killing it again says so rather than reporting success, which is the
	// check busybox makes with GetExitCodeProcess before terminating anything.
	if _, _, status := runKill(t, "kill "+strconv.Itoa(pid)+"\n"); status == 0 {
		t.Fatal("killing a dead process reported success")
	}
}

// sleepingHelper builds this shell, which is the only long-running program the
// test can be sure exists on the machine.
func sleepingHelper(t *testing.T) string {
	t.Helper()
	binary := t.TempDir() + "/helper"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "github.com/xiongnemo/nemosh/cmd/nemosh")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the helper: %v\n%s", err, output)
	}
	return binary
}
