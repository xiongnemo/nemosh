package runtime

import (
	"bytes"
	"context"
	"path/filepath"
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
	// The shell's descriptors as they were at launch, each by its own route: a file inherited
	// as a handle; a heredoc moved into a file the shell reads too, so the job and the shell
	// share one offset; and a buffer -- the test's stdout -- through a pipe the parent drains.
	// The last job inherits the heredoc again, which is where two copies of it once raced.
	// An `exec >` after the launch does not take the job's output with it. busybox-w32 and
	// bash 5.3 answer the same.
	t.Run("the job has the shell's descriptors from its launch", func(t *testing.T) {
		file := filepath.ToSlash(filepath.Join(t.TempDir(), "three"))
		script := "f='" + file + "'\nexec 3>\"$f\"\necho three >&3 & wait\nexec 3>&-\necho \"3=[$(cat \"$f\")]\"\n" +
			"exec 4<<'DOC'\nline1\nline2\nDOC\n{ read line <&4; echo \"4=[$line]\"; } & wait\nread line <&4; echo \"shell=[$line]\"\n" +
			"{ sleep 0.3; echo from-job; } &\nexec 7>&1 >\"$f\"\necho into-file\nwait\nexec 1>&7\necho \"file=[$(cat \"$f\")]\"\n"
		stdout, stderr := run(t, script)
		if stdout != "3=[three]\n4=[line1]\nshell=[line2]\nfrom-job\nfile=[into-file]\n" {
			t.Fatalf("stdout %q stderr %q", stdout, stderr)
		}
	})
	// `$$` and `$PPID` are the shell's in a job, in both references; `$BASHPID` is the
	// job's own, as bash gives it, and the same pid `$!` names.
	t.Run("a job keeps the shell's $$ and $PPID, and has its own $BASHPID", func(t *testing.T) {
		stdout, stderr := run(t, "d='"+filepath.ToSlash(t.TempDir())+"'\n"+
			"top=\"$$ $PPID $BASHPID\"\n{ echo \"$$ $PPID $BASHPID\" > \"$d/job\"; } & p=$!; wait\n"+
			"read s pp b < \"$d/job\"\nset -- $top\n[ \"$s $pp\" = \"$1 $2\" ] && echo shell-kept\n"+
			"[ \"$b\" = \"$p\" ] && [ \"$b\" != \"$s\" ] && echo own-bashpid\n[ \"$3\" = \"$1\" ] && echo top-same\n")
		if stdout != "shell-kept\nown-bashpid\ntop-same\n" {
			t.Fatalf("stdout %q stderr %q", stdout, stderr)
		}
	})
	t.Run("jobs -l and -p name the pids", func(t *testing.T) {
		d := filepath.ToSlash(t.TempDir())
		stdout, stderr := run(t, "sleep 2 & a=$!\njobs -p > '"+d+"/p'\njobs -l > '"+d+"/l'\n"+
			"read p < '"+d+"/p'\n[ \"$p\" = \"$a\" ] && echo p-ok\n"+
			"read l < '"+d+"/l'\n[ \"$l\" = \"[1] $a Running\" ] && echo l-ok\nkill %1; wait\n")
		if stdout != "p-ok\nl-ok\n" {
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
