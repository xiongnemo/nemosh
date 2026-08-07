//go:build windows

package runtime_test

import (
	"strings"
	"testing"
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

func TestRuntime_keepsTheDriveAliasLowercase_whenCaseIsResolved(t *testing.T) {
	// The alias is Nemosh's own spelling rather than the filesystem's, so it
	// stays lowercase however the drive letter was typed.
	// When
	status, stdout, _, _ := runCdScript(t, "mkdir Sub\ncd Sub\npwd\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	if !strings.HasPrefix(stdout, "/") {
		t.Fatalf("pwd = %q, want the posix-drive display form", stdout)
	}
	if head := strings.SplitN(strings.TrimPrefix(stdout, "/"), "/", 2)[0]; head != strings.ToLower(head) {
		t.Fatalf("pwd = %q, want the drive alias lowercase", stdout)
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
