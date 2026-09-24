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
	directory, err := os.MkdirTemp("", "nemosh-job-binary-")
	if err == nil {
		if binary, built := buildJobBinary(directory); built {
			jobExecutable = func() (string, error) { return binary, nil }
		}
	}
	code := m.Run()
	if err == nil {
		_ = os.RemoveAll(directory)
	}
	os.Exit(code)
}

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
