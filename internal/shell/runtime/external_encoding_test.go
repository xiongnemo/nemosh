package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	shellruntime "github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// runHelperWith runs the test binary as an external command with the given
// environment and helper arguments, and returns its status and stdout.
func runHelperWith(t *testing.T, environ []string, arguments string) (int, string, string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}
	var stdout, stderr bytes.Buffer
	rt := shellruntime.NewWithState(applets.DefaultRegistry, shellruntime.Streams{Stdout: &stdout, Stderr: &stderr},
		shellruntime.State{
			Cwd: shellruntime.WorkingDirectory(t.TempDir()),
			Env: shellruntime.NewEnvironment(append([]string{"NEMOSH_RUNTIME_HELPER_PROCESS=1"}, environ...)),
		})
	status := rt.RunScript(context.Background(),
		filepath.ToSlash(executable)+" -test.run=TestRuntimeHelperProcess -- "+arguments+"\n")
	return status, stdout.String(), stderr.String()
}

// Nemosh holds argv as UTF-8 and Windows hands it to a child as UTF-16, so a
// non-ASCII operand crosses two conversions on the way out and two more coming
// back through the child's stdout. U+1F30F is there because it needs a
// surrogate pair, which a naive UTF-16 length or index would split.
func TestRuntime_carriesNonASCIIArgumentsToAChildUnchanged(t *testing.T) {
	// Given / When
	status, stdout, stderr := runHelperWith(t, nil, "argv 参数 🌏 naïve")

	// Then
	if status != 0 {
		t.Fatalf("expected the child to run, status=%d stderr=%q", status, stderr)
	}
	if want := "参数\n🌏\nnaïve\n"; stdout != want {
		t.Errorf("child saw %q, want %q", stdout, want)
	}
}

// The child environment block is built separately from argv
// (environment_windows_names.go), so it needs its own pin.
func TestRuntime_carriesNonASCIIEnvironmentValuesToAChildUnchanged(t *testing.T) {
	// Given / When
	status, stdout, stderr := runHelperWith(t, []string{"NEMOSH_CHILD_VALUE=值 🌏"}, "state")

	// Then
	if status != 0 {
		t.Fatalf("expected the child to run, status=%d stderr=%q", status, stderr)
	}
	// The helper prints its working directory first, then the value.
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if got := lines[len(lines)-1]; got != "值 🌏" {
		t.Errorf("child saw %q, want %q", got, "值 🌏")
	}
}
