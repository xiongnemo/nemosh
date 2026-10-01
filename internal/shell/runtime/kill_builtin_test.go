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

	// Then: a `wait` with no operands says nothing of how it ended, as busybox's says nothing
	// -- `wait %1` names it -- and nothing else is said
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

	// Then: the job is still there to be waited for, and its status is the
	// signal's -- 143, TERM's, which is what both references' `wait` answers. What
	// must not happen is `wait` failing to find a job that kill only signalled.
	if stderr != "Terminated\n" {
		t.Fatalf("wait complained after kill: %s", stderr)
	}
	if status != 143 {
		t.Fatalf("status = %d, want 143, 128 plus TERM", status)
	}
}

// With busybox's statuses: a job spec that names no job is 2, its getjob error, and any other
// failure counts one towards the status.
func TestKill_refusesWhatItCannotDo(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
		status int
	}{
		{name: "no such job", script: "kill %9\n", want: "kill: %9: no such job", status: 2},
		{name: "not a job and not a number", script: "kill nope\n", want: "kill: invalid pid 'nope'", status: 1},
		{name: "job zero", script: "kill %0\n", want: "%0: no such job", status: 2},
		// 1, which both references answer; this used to be 2.
		{name: "an unknown signal", script: "kill -BOGUS 1\n", want: "kill: invalid signal 'BOGUS'", status: 1},
		{name: "-s and nothing after it", script: "kill -s\n", want: "kill: invalid signal 's'", status: 1},
		{name: "nothing at all", script: "kill\n", want: "kill: expected a job or a process id", status: 1},
		{name: "a signal and nothing to send it to", script: "kill -9\n", want: "expected a job or a process id", status: 1},
		{name: "one count for each operand that failed", script: "kill nope 99999999 never\n", want: "kill: invalid pid 'never'", status: 3},
		{name: "a pid with no process", script: "kill 99999999\n", want: "kill: cannot signal pid 99999999: No such process", status: 1},
		{name: "-- ends the options", script: "kill -- nope\n", want: "kill: invalid pid 'nope'", status: 1},
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

// kill, jobs and wait say which job a spec meant when there is none, "No current job" or "No
// previous job", as every ash does, busybox's and dash among them. Each said "%%: no such job".
func TestJobSpec_aSpecThatNamesNoJobSaysWhichOneIsMissing(t *testing.T) {
	for script, want := range map[string]string{
		"kill %%\n": "kill: No current job\n",
		"jobs %-\n": "jobs: No previous job\n",
		"wait %+\n": "wait: No current job\n",
		"wait %7\n": "wait: %7: no such job\n",
	} {
		if _, stderr, _ := runKill(t, script); stderr != want {
			t.Errorf("%q: stderr = %q, want %q", script, stderr, want)
		}
	}
}

// Every job spec is found before anything is sent, as busybox's killcmd finds them: one that
// names no job leaves the job that is there alone. `%1` was killed, and the status was 1.
func TestKill_aJobSpecThatNamesNoJobSendsNothing(t *testing.T) {
	// When
	stdout, stderr, _ := runKill(t, "sleep 30 &\nkill %1 %9\necho \"st=$?\"\nkill -0 %1 && echo alive\nkill %1\nwait\n")

	// Then
	if !strings.HasPrefix(stdout, "st=2\nalive\n") || !strings.Contains(stderr, "kill: %9: no such job") {
		t.Fatalf("stdout = %q, stderr = %q; want st=2, the job alive, and %%9 named", stdout, stderr)
	}
}

// Both spellings of a signal, because a script writes the number and a person
// writes the name, and refusing either refuses half the users.
//
// And the job reports which one ended it: `wait %1` answers 128 plus the signal,
// and names it on stderr as busybox's does.
func TestKill_acceptsASignalEitherWay(t *testing.T) {
	for _, test := range []struct {
		signal string
		word   string
		status int
	}{
		{signal: "-9", word: "Killed", status: 137},
		{signal: "-KILL", word: "Killed", status: 137},
		{signal: "-TERM", word: "Terminated", status: 143},
		{signal: "-SIGTERM", word: "Terminated", status: 143},
		{signal: "-15", word: "Terminated", status: 143},
		{signal: "-HUP", word: "Hangup", status: 129},
		// An interrupt is not announced, in either reference.
		{signal: "-INT", word: "", status: 130},
	} {
		t.Run(test.signal, func(t *testing.T) {
			stdout, stderr, _ := runKill(t, "sleep 30 &\nkill "+test.signal+" %1\nwait %1\necho $?\n")
			if want := strconv.Itoa(test.status) + "\n"; stdout != want {
				t.Fatalf("wait answered %q, want %q", stdout, want)
			}
			if strings.TrimSuffix(stderr, "\n") != test.word {
				t.Fatalf("stderr = %q, want %q", stderr, test.word)
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
	// Without SIG, as busybox lists them.
	for _, want := range []string{" 1) HUP\n", " 9) KILL\n", "15) TERM\n"} {
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
	// A name of its own: the applets package's pkill test matches its helper by name, and
	// ended this one when the two packages' tests ran at once and both were "helper".
	binary := t.TempDir() + "/killprobe"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "github.com/xiongnemo/nemosh/cmd/nemosh")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the helper: %v\n%s", err, output)
	}
	return binary
}
