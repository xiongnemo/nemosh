package runtime

import (
	"bytes"
	"context"
	"os"
	goruntime "runtime"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A program a signal ended leaves 128+n, as in both references. The program is a nemosh its
// own background job kills: on Windows, TerminateProcess with the signal in the exit code,
// 0x0F000000, and $? was that number; busybox-w32 reads it as TERM's, 143, and says
// `Terminated`, as it says the signal of any command it waited for but INT's and PIPE's.
func TestProgramStatus_aProgramEndedBySignalLeaves128PlusN(t *testing.T) {
	binary := os.Getenv(jobBinaryVariable)
	if binary == "" {
		t.Skip("no nemosh was built for jobs to run")
	}
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	// Its job is a process whatever this run's launcher, since a goroutine job's `kill $$`
	// raises the signal in its shell rather than ending the process it runs in.
	rt.RunScript(context.Background(), "n="+singleQuoteForReuse(binary)+"\nNEMOSH_JOBS=process \"$n\" -c 'kill $$ & sleep 5'\necho \"status $?\"\n")

	// Then
	if stdout.String() != "status 143\n" {
		t.Fatalf("stdout = %q, stderr = %q, want status 143", stdout.String(), stderr.String())
	}
	if goruntime.GOOS == "windows" && stderr.String() != "Terminated\n" {
		t.Fatalf("stderr = %q, want %q", stderr.String(), "Terminated\n")
	}
}
