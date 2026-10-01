package runtime_test

import (
	"strings"
	"testing"
)

// jobsLine is one line of `jobs` as both references write it: the text padded to column 33,
// where the command would follow.
func jobsLine(text string) string {
	return text + strings.Repeat(" ", 33-len(text)) + "\n"
}

// The job table as busybox keeps it. A new job takes the lowest number free, so after %1 is
// reaped the next job is %1 again. The current job, %% or %+, is the one started last, and
// the previous, %-, the one before it; `jobs` lists them newest first, marked + and -, each
// padded to column 33 where the command would follow. A job spec that names no job is "no
// such job". bash's -r and -s, which busybox lacks, list the running jobs and the stopped
// ones -- and nothing here is ever stopped.
func TestRuntime_jobsTableAsBusyboxKeepsIt(t *testing.T) {
	line := jobsLine
	tests := []struct {
		script, want string
	}{
		{
			"sleep 5 & sleep 5 & sleep 5 &\njobs\nkill %1 %2 %3; wait",
			line("[3]+  Running") + line("[2]-  Running") + line("[1]   Running"),
		},
		{
			"sleep 5 & sleep 5 & sleep 5 &\nkill %1; wait %1\nsleep 5 &\njobs\njobs -p | wc -l\nkill %1 %2 %3; wait",
			line("[1]+  Running") + line("[3]-  Running") + line("[2]   Running") + "3\n",
		},
		{
			"sleep 5 & sleep 5 &\nkill %-; wait %-; echo \"prev=$?\"\nkill %%; wait %%; echo \"cur=$?\"\njobs; echo \"[$(jobs)]\"",
			"prev=143\ncur=143\n[]\n",
		},
		{
			"sleep 5 & sleep 5 &\njobs %1; jobs %9 2>/dev/null; echo \"st=$?\"\nkill %+ %1; wait",
			line("[1]-  Running") + "st=2\n",
		},
		{
			"sleep 5 &\njobs -r; jobs -s; echo \"st=$?\"\nkill %%; wait %%\nkill %% 2>/dev/null; echo \"none=$?\"",
			// `kill %%` once there is no job is "No current job" and 2, busybox's getjob error.
			line("[1]+  Running") + "st=0\nnone=2\n",
		},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0", stdout, status, test.want)
			}
		})
	}
}
