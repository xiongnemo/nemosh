package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// TestMain lets this test binary be the child of a background job that is a process: a job
// launched here runs os.Executable, which is this binary, as `--job <handle>`, and cmd/nemosh
// is not linked into it. Before the tests, so the child never runs them. With
// NEMOSH_JOBS=process in the environment, every background job the suite starts takes this
// path, which is how the goroutine's tests hold the process to the same answers.
func TestMain(m *testing.M) {
	if len(os.Args) == 3 && os.Args[1] == "--job" {
		os.Exit(runJobChild(os.Args[2]))
	}
	os.Exit(m.Run())
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
