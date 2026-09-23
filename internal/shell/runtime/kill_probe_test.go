package runtime

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// `kill -0` asks whether something is alive, and must not change the answer by asking.
//
// It did, twice over. For a job, zero was one more signal, and every signal cancels -- so
// `kill -0 $!` stopped the job it was checking on. For a pid, zero reached TerminateProcess
// like every other number and ended the process with exit code 0. Both references answer
// the question and leave the target running.
func TestKill_zeroAsksAboutAJobWithoutStoppingIt(t *testing.T) {
	// Given / When: a job that says something after a pause, probed at once
	stdout, stderr, status := runKill(t, "(sleep 0.3; echo alive) &\nkill -0 %1\necho \"probe=$?\"\nwait\n")

	// Then: the probe said yes, and the job lived to finish
	if stdout != "probe=0\nalive\n" || stderr != "" || status != 0 {
		t.Fatalf("stdout = %q, stderr = %q, status = %d; want the probe to succeed and the job to finish",
			stdout, stderr, status)
	}
}

// The loop it is written for ends. `while kill -0 $pid` used to run forever, because the
// probe kept succeeding for as long as the job's record lasted, after the job itself was
// gone. `$!` is `%N` here, so the answer a pid gets once its process has exited is the one
// the job has to give.
func TestKill_zeroSaysNoOnceTheJobHasEnded(t *testing.T) {
	// Given: a job that ends at once
	script := "true &\np=$!\nn=0\nwhile kill -0 $p 2>/dev/null && [ $n -lt 100 ]; do sleep 0.05; n=$((n+1)); done\necho \"polls=$n\"\nkill -0 $p\necho \"after=$?\"\n"

	// When
	stdout, stderr, _ := runKill(t, script)

	// Then: the loop ended well before its bound, and a direct probe says why
	if strings.Contains(stdout, "polls=100") {
		t.Fatalf("the probe never said the job had ended:\n%s", stdout)
	}
	if !strings.Contains(stdout, "after=1") || !strings.Contains(stderr, "already ended") {
		t.Fatalf("stdout = %q, stderr = %q; want a failed probe that says the job has ended", stdout, stderr)
	}
}

// STOP and CONT are refused by name. Delivered the way every signal is here, STOP would end
// the job it was meant to pause -- and that is what it used to do.
func TestKill_refusesToSuspendAndLeavesTheJobAlone(t *testing.T) {
	for _, signal := range []string{"STOP", "SIGCONT", "19", "TSTP"} {
		t.Run(signal, func(t *testing.T) {
			// When
			stdout, stderr, _ := runKill(t, "(sleep 0.3; echo alive) &\nkill -"+signal+" %1\necho \"st=$?\"\nwait\n")

			// Then
			if stdout != "st=1\nalive\n" {
				t.Fatalf("stdout = %q; want a refusal and the job still finishing", stdout)
			}
			if !strings.Contains(stderr, "suspend") {
				t.Fatalf("stderr = %q; want it to say why", stderr)
			}
		})
	}
}

// And for a real process: the probe leaves it running, and says no once it is gone.
func TestKill_zeroAsksAboutAProcessWithoutEndingIt(t *testing.T) {
	// Given: a child that would outlive the test
	child := exec.Command(sleepingHelper(t), "-c", "sleep 60")
	if err := child.Start(); err != nil {
		t.Fatalf("starting the helper: %v", err)
	}
	defer func() { _ = child.Process.Kill() }()
	pid := strconv.Itoa(child.Process.Pid)
	waited := make(chan error, 1)
	go func() { waited <- child.Wait() }()

	// When
	_, stderr, status := runKill(t, "kill -0 "+pid+"\n")

	// Then: yes, and it is still there to be asked about
	if stderr != "" || status != 0 {
		t.Fatalf("kill -0 %s: stderr = %q, status = %d", pid, stderr, status)
	}
	select {
	case err := <-waited:
		t.Fatalf("kill -0 ended the process it asked about (%v)", err)
	case <-time.After(500 * time.Millisecond):
	}

	// And once it has exited the probe says so.
	_ = child.Process.Kill()
	select {
	case <-waited:
	case <-time.After(20 * time.Second):
		t.Fatal("the helper would not die")
	}
	if _, _, status := runKill(t, "kill -0 "+pid+"\n"); status == 0 {
		t.Fatal("kill -0 said yes about a process that has exited")
	}
}
