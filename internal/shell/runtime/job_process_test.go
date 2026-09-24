package runtime

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Background jobs as processes, under NEMOSH_JOBS=process. The job processes are this test
// binary, which TestMain turns into the child when it is started with --job.
func TestProcessJobs_behaveAsTheReferencesDo(t *testing.T) {
	run := func(t *testing.T, script string) (string, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.env.Set("NEMOSH_JOBS", "process")
		status := rt.RunScript(context.Background(), script)
		rt.CloseBatch(status)
		return stdout.String(), stderr.String()
	}

	t.Run("$! is a pid and wait takes it", func(t *testing.T) {
		stdout, stderr := run(t, "true & p=$!\ncase $p in *[!0-9]*|'') echo not-a-pid;; *) echo pid;; esac\nwait $p; echo \"st=$?\"\n(exit 7) & wait $!; echo \"st=$?\"\n")
		if stdout != "pid\nst=0\nst=7\n" {
			t.Fatalf("stdout %q stderr %q", stdout, stderr)
		}
	})
	t.Run("the job has the shell's functions, variables and arrays", func(t *testing.T) {
		stdout, stderr := run(t, "x=inner; a=(p q); declare -A m=([k]=v)\nf() { echo \"f $x ${a[1]} ${m[k]} $1\"; }\nf arg & wait\n")
		if stdout != "f inner q v arg\n" {
			t.Fatalf("stdout %q stderr %q", stdout, stderr)
		}
	})
	t.Run("kill by pid and by job, and the status says which signal", func(t *testing.T) {
		stdout, stderr := run(t, "sleep 5 & p=$!\nkill $p; wait $p; echo \"pid=$?\"\nsleep 5 & kill %2; wait %2; echo \"job=$?\"\n")
		if stdout != "pid=143\njob=143\n" || !strings.Contains(stderr, "Terminated") {
			t.Fatalf("stdout %q stderr %q", stdout, stderr)
		}
	})
	t.Run("a list item in the background", func(t *testing.T) {
		stdout, stderr := run(t, "echo one && echo two & wait\necho after\n")
		if stdout != "one\ntwo\nafter\n" {
			t.Fatalf("stdout %q stderr %q", stdout, stderr)
		}
	})
	t.Run("the pid is a real process", func(t *testing.T) {
		stdout, stderr := run(t, "sleep 2 & echo $!\nkill -0 $! && echo alive\nkill $!\nwait\n")
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 2 || lines[1] != "alive" {
			t.Fatalf("stdout %q stderr %q", stdout, stderr)
		}
		if _, err := strconv.Atoi(lines[0]); err != nil {
			t.Fatalf("$! = %q, want a number", lines[0])
		}
	})
}

func TestJobExitStatus_readsASignalInTheTopByte(t *testing.T) {
	for code, want := range map[uint32]int{0: 0, 7: 7, 15 << 24: 143, 9 << 24: 137, 256 + 3: 3} {
		if got := jobExitStatus(code); got != want {
			t.Errorf("jobExitStatus(%#x) = %d, want %d", code, got, want)
		}
	}
}
