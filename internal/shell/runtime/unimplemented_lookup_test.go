package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	shellruntime "github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// Dispatch refuses an unimplemented builtin before it ever looks at PATH, so
// `command -v` and `which` must refuse it too. They did not: both fell straight
// through to the PATH search, so a directory holding a program of that name made
// them answer "yes, here it is" about a name that comes back 126 when run.
//
// This was invisible on Windows and Linux by luck -- neither ships a file called
// `ulimit`, `hash`, `fg`, or `bg` -- and macOS found it the first time the test
// matrix ran there, because macOS does ship /usr/bin/ulimit. The fixture below
// recreates that condition on any platform rather than waiting for the next
// operating system to disagree.
func TestRuntime_refusesAnUnimplementedBuiltin_fromCommandVandWhich_evenWhenPathHasOne(t *testing.T) {
	for _, name := range []string{"ulimit", "hash", "fg", "bg"} {
		t.Run(name, func(t *testing.T) {
			// Given a directory on PATH that really does hold a runnable
			// program of that name.
			directory := t.TempDir()
			copyRuntimeHelper(t, filepath.Join(directory, executableName(name)))
			var stdout, stderr bytes.Buffer
			rt, err := shellruntime.NewRuntimeWithState(applets.DefaultRegistry,
				shellruntime.Streams{Stdout: &stdout, Stderr: &stderr},
				shellruntime.State{
					Cwd: shellruntime.WorkingDirectory(directory),
					Env: shellruntime.NewEnvironment([]string{"PATH=" + directory}),
				})
			if err != nil {
				t.Fatalf("construct runtime: %v", err)
			}

			// When
			status := rt.RunScript(context.Background(),
				"command -v "+name+"\necho command=$?\nwhich "+name+"\necho which=$?\n")

			// Then both must say no, and say nothing else: a script asks these
			// before relying on a name, and the honest answer is that this shell
			// will not run it.
			if status != 0 {
				t.Fatalf("script status = %d, stderr = %q", status, stderr.String())
			}
			want := "command=1\nwhich=1\n"
			if stdout.String() != want {
				t.Fatalf("stdout = %q, want %q", stdout.String(), want)
			}
		})
	}
}

// The refusal must not swallow a name the shell can genuinely run, which is the
// failure mode of over-correcting the above.
func TestRuntime_stillReportsARealProgramFromCommandV(t *testing.T) {
	// Given
	directory := t.TempDir()
	program := filepath.Join(directory, executableName("nemosh-lookup-probe"))
	copyRuntimeHelper(t, program)
	var stdout, stderr bytes.Buffer
	rt, err := shellruntime.NewRuntimeWithState(applets.DefaultRegistry,
		shellruntime.Streams{Stdout: &stdout, Stderr: &stderr},
		shellruntime.State{
			Cwd: shellruntime.WorkingDirectory(directory),
			Env: shellruntime.NewEnvironment([]string{"PATH=" + directory}),
		})
	if err != nil {
		t.Fatalf("construct runtime: %v", err)
	}

	// When
	status := rt.RunScript(context.Background(), "command -v nemosh-lookup-probe\necho status=$?\n")

	// Then
	if status != 0 {
		t.Fatalf("script status = %d, stderr = %q", status, stderr.String())
	}
	want := filepath.ToSlash(program) + "\nstatus=0\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	_ = os.Remove(program)
}
