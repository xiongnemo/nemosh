package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// TestMain gives a background job that is a process something to run. The job starts
// jobExecutable as `--job <handle>`, and the one this test binary would find is itself,
// which cmd/nemosh is not linked into. So a nemosh is built first, once, and the jobs run
// that: the real binary, without -race, which starts in milliseconds where this binary
// under -race takes more than a second. A suite of a few hundred jobs took eight minutes
// the other way.
//
// If the build cannot happen -- no go command on PATH -- the jobs run this binary instead,
// which the intercept below turns into the child before any test starts.
func TestMain(m *testing.M) {
	if len(os.Args) == 3 && os.Args[1] == "--job" {
		os.Exit(runJobChild(os.Args[2]))
	}
	// Either way this binary can be a job's: the built nemosh, or itself through the
	// intercept above.
	AllowJobProcesses()
	endJobsWithTheTestBinary()
	// A test that runs this binary again, as a helper, has it use the nemosh already
	// built rather than build its own. The build is seconds on a CI runner, and a helper
	// held to a five-second deadline ran out of it there on 2026-09-27.
	if binary := os.Getenv(jobBinaryVariable); binary != "" {
		if _, err := os.Stat(binary); err == nil {
			jobExecutable = func() (string, error) { return binary, nil }
			os.Exit(m.Run())
		}
	}
	directory, err := os.MkdirTemp("", "nemosh-job-binary-")
	if err == nil {
		if binary, built := buildJobBinary(directory); built {
			jobExecutable = func() (string, error) { return binary, nil }
			_ = os.Setenv(jobBinaryVariable, binary)
		}
	}
	code := m.Run()
	if err == nil {
		_ = os.RemoveAll(directory)
	}
	os.Exit(code)
}

// jobBinaryVariable names, for a copy of this binary run as a helper, the nemosh the
// first one built.
const jobBinaryVariable = "NEMOSH_TEST_JOB_BINARY"

func buildJobBinary(directory string) (string, bool) {
	binary := filepath.Join(directory, "nemosh")
	if goruntime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "github.com/xiongnemo/nemosh/cmd/nemosh")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "job binary not built, so jobs run the test binary: %v\n%s", err, output)
		return "", false
	}
	return binary, true
}

func runJobChild(argument string) int {
	file, err := JobStateFile(argument)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	data, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	rt := New(applets.DefaultRegistry, Streams{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr})
	return rt.RunJob(context.Background(), data)
}
