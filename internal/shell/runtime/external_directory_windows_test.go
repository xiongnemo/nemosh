//go:build windows

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

// directoryOfWideLength builds a real directory whose native path is exactly
// units UTF-16 characters long. Everything here is ASCII, so a character is a
// byte; the wide-character distinction is exercised by the unit tests.
func directoryOfWideLength(t *testing.T, units int) string {
	t.Helper()
	root := t.TempDir()
	remaining := units - len(root) - 1
	if remaining < 1 {
		t.Fatalf("temporary root is already %d characters, wanted room inside %d", len(root), units)
	}
	dir := filepath.Join(root, strings.Repeat("d", remaining))
	if len(dir) != units {
		t.Fatalf("built a %d character directory, wanted %d", len(dir), units)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create %d character directory: %v", units, err)
	}
	return dir
}

// runHelperFrom runs the test binary as an external command with cwd as the
// shell's working directory, and returns the working directory the child sees.
func runHelperFrom(t *testing.T, cwd string) (int, string, string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}
	var stdout, stderr bytes.Buffer
	rt := shellruntime.NewWithState(applets.DefaultRegistry, shellruntime.Streams{Stdout: &stdout, Stderr: &stderr},
		shellruntime.State{
			Cwd: shellruntime.WorkingDirectory(cwd),
			Env: shellruntime.NewEnvironment([]string{"NEMOSH_RUNTIME_HELPER_PROCESS=1"}),
		})
	status := rt.RunScript(context.Background(),
		filepath.ToSlash(executable)+" -test.run=TestRuntimeHelperProcess -- state\n")
	childCwd, _, _ := strings.Cut(stdout.String(), "\n")
	return status, childCwd, stderr.String()
}

func TestRuntime_launchesAnExternal_fromAWorkingDirectoryLongerThanCreateProcessAccepts(t *testing.T) {
	// Given a working directory past the MAX_PATH-shaped lpCurrentDirectory limit.
	deep := directoryOfWideLength(t, 310)

	// When
	status, childCwd, stderr := runHelperFrom(t, deep)

	// Then the child launches, from that same directory under another name.
	if status != 0 {
		t.Fatalf("expected the child to launch, status=%d stderr=%q", status, stderr)
	}
	want, err := os.Stat(deep)
	if err != nil {
		t.Fatalf("stat the deep directory: %v", err)
	}
	got, err := os.Stat(childCwd)
	if err != nil {
		t.Fatalf("stat the child's working directory %q: %v", childCwd, err)
	}
	if !os.SameFile(want, got) {
		t.Errorf("child ran in %q, want the same directory as %q", childCwd, deep)
	}
}

func TestRuntime_leavesTheWorkingDirectoryAlone_whenCreateProcessAcceptsItAsWritten(t *testing.T) {
	// Given the longest working directory CreateProcess takes.
	limit := directoryOfWideLength(t, 258)

	// When
	status, childCwd, stderr := runHelperFrom(t, limit)

	// Then it is handed over verbatim rather than shortened.
	if status != 0 {
		t.Fatalf("expected the child to launch, status=%d stderr=%q", status, stderr)
	}
	if childCwd != limit {
		t.Errorf("child ran in %q, want the directory unchanged at %q", childCwd, limit)
	}
}
