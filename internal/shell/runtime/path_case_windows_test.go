//go:build windows

package runtime_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// windows-path-model.md asks shell-generated display paths to carry the real
// on-disk case and to fall back silently to the spelling as typed. Only the
// fallback half held before: `cd /c/users/NEMO` then `pwd` echoed that back.
func TestRuntime_reportsTheOnDiskCase_afterCdWithADifferentSpelling(t *testing.T) {
	// When a directory is made with one case and entered with another.
	status, stdout, stderr, _ := runCdScript(t, "mkdir MixedCaseDir\ncd mixedcasedir\npwd\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
	}
	if !strings.HasSuffix(strings.TrimSpace(stdout), "MixedCaseDir") {
		t.Fatalf("pwd = %q, want it to end in the on-disk spelling MixedCaseDir", stdout)
	}
}

func TestRuntime_correctsEveryComponent_notOnlyTheLast(t *testing.T) {
	// When
	status, stdout, stderr, _ := runCdScript(t, "mkdir -p Outer/Inner\ncd outer/inner\npwd\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
	}
	if !strings.HasSuffix(strings.TrimSpace(stdout), "Outer/Inner") {
		t.Fatalf("pwd = %q, want both components corrected", stdout)
	}
}

// The drive is Nemosh's own spelling rather than the filesystem's, so it is C: however the
// letter was typed, as busybox-w32 writes it, and under NEMOSH_PATHS=msys the alias stays /c.
func TestRuntime_keepsTheDriveLetterCase_whenCaseIsResolved(t *testing.T) {
	for _, testCase := range []struct {
		env     string
		spelled func(string) bool
	}{
		{"NEMOSH_PATHS=", driveSpelled},
		{"NEMOSH_PATHS=msys", aliasSpelled},
	} {
		// When the drive is typed in the other case
		dir := t.TempDir()
		typed := strings.ToLower(dir[:1]) + dir[1:]
		var stdout bytes.Buffer
		rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout},
			runtime.State{Cwd: runtime.WorkingDirectory(dir), Env: runtime.NewEnvironment([]string{testCase.env})})
		status := rt.RunScript(context.Background(), "mkdir Sub\ncd '"+typed+"/sub'\npwd\n")

		// Then
		if status != 0 || !testCase.spelled(stdout.String()) || !strings.HasSuffix(stdout.String(), "/Sub\n") {
			t.Fatalf("%s: status = %d, pwd = %q", testCase.env, status, stdout.String())
		}
	}
}

func TestRuntime_keepsTheSpelling_whenTheDirectoryCannotBeAsked(t *testing.T) {
	// A path that does not exist is never reached by cd, so this pins the
	// fallback: an unanswerable path leaves the working directory alone rather
	// than turning into a diagnostic about something nobody asked for.
	// When
	status, stdout, stderr, _ := runCdScript(t,
		"start=$(pwd)\ncd nosuchdir\ntest \"$(pwd)\" = \"$start\" && echo unchanged\n")

	// Then
	if status != 0 || stdout != "unchanged\n" {
		t.Fatalf("status = %d, stdout = %q, stderr = %q, want the directory unchanged", status, stdout, stderr)
	}
}
