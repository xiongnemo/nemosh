package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// runCdScript runs source in a fresh temporary directory so `pwd` and `$PWD`
// have something stable to answer with.
func runCdScript(t *testing.T, source string) (int, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr},
		runtime.State{Cwd: runtime.WorkingDirectory(dir)})
	status := rt.RunScript(context.Background(), source)
	return status, stdout.String(), stderr.String(), dir
}

func TestRuntime_tracksTheWorkingDirectory_whenPwdVariableIsRead(t *testing.T) {
	// $PWD used to keep whatever the process started with, and that stale value
	// was handed to every child.
	// When
	status, stdout, _, _ := runCdScript(t, "mkdir sub\ncd sub\ntest \"$PWD\" = \"$(pwd)\" && echo matched\n")

	// Then
	if status != 0 || stdout != "matched\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "matched\n")
	}
}

func TestRuntime_recordsThePreviousDirectory_whenCdSucceeds(t *testing.T) {
	// When
	status, stdout, _, _ := runCdScript(t, "mkdir sub\nstart=$(pwd)\ncd sub\ntest \"$OLDPWD\" = \"$start\" && echo matched\n")

	// Then
	if status != 0 || stdout != "matched\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "matched\n")
	}
}

func TestRuntime_goesBack_whenCdIsGivenALoneDash(t *testing.T) {
	// busybox ash reads a lone dash as OLDPWD and prints where it landed
	// (cdcmd, shell/ash.c:11823).
	// When
	status, stdout, stderr, _ := runCdScript(t, "mkdir sub\nstart=$(pwd)\ncd sub\ncd -\ntest \"$(pwd)\" = \"$start\" && echo matched\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
	}
	if !strings.HasSuffix(stdout, "matched\n") {
		t.Fatalf("stdout = %q, want it to end in %q", stdout, "matched\n")
	}
	if !strings.Contains(stdout, "sub") && len(strings.Split(strings.TrimSuffix(stdout, "matched\n"), "\n")) < 2 {
		t.Fatalf("stdout = %q, want cd - to print the directory it landed in", stdout)
	}
}

func TestRuntime_goesHome_whenCdHasNoOperand(t *testing.T) {
	// When
	status, stdout, stderr, dir := runCdScript(t, "mkdir sub\nHOME=$(pwd)/sub\ncd\ntest \"$(pwd)\" = \"$HOME\" && echo matched\n")

	// Then
	if status != 0 || stdout != "matched\n" {
		t.Fatalf("status = %d, stdout = %q, stderr = %q, want 0 and %q (dir %s)", status, stdout, stderr, "matched\n", dir)
	}
}

func TestRuntime_reportsHomeUnset_whenCdHasNoOperandAndNoHome(t *testing.T) {
	// When
	status, _, stderr, _ := runCdScript(t, "unset HOME\ncd\n")

	// Then
	if status != 1 || !strings.Contains(stderr, "HOME not set") {
		t.Fatalf("status = %d, stderr = %q, want 1 and a HOME-not-set diagnostic", status, stderr)
	}
}

func TestRuntime_reportsOldpwdUnset_whenCdIsGivenADashWithoutOne(t *testing.T) {
	// When
	status, _, stderr, _ := runCdScript(t, "cd -\n")

	// Then
	if status != 1 || !strings.Contains(stderr, "OLDPWD not set") {
		t.Fatalf("status = %d, stderr = %q, want 1 and an OLDPWD-not-set diagnostic", status, stderr)
	}
}

func TestRuntime_namesTheReason_whenCdTargetsARegularFile(t *testing.T) {
	// The old code formatted a nil error here, so the diagnostic ended in the
	// literal text <nil>.
	// Given
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plain.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	var stdout, stderr bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr},
		runtime.State{Cwd: runtime.WorkingDirectory(dir)})

	// When
	status := rt.RunScript(context.Background(), "cd plain.txt\n")

	// Then
	if status != 1 {
		t.Fatalf("status = %d, want 1", status)
	}
	if strings.Contains(stderr.String(), "<nil>") {
		t.Fatalf("stderr = %q, want a real reason rather than a formatted nil", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Not a directory") {
		t.Fatalf("stderr = %q, want it to say the target is not a directory", stderr.String())
	}
}

func TestRuntime_leavesTheDirectoryAlone_whenCdFails(t *testing.T) {
	// When
	status, stdout, _, _ := runCdScript(t, "start=$(pwd)\ncd nosuchdir\ntest \"$(pwd)\" = \"$start\" && echo unchanged\n")

	// Then
	if status != 0 || stdout != "unchanged\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "unchanged\n")
	}
}
