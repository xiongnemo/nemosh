package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A failure on one operand must not abandon the operands after it.
//
// POSIX has rm continue with the remaining operands, and busybox does:
// measured, `rm a missing c` reports all three and exits 1. This reported only
// the first failure and left `c` in place, which in a cleanup script means the
// tree is half removed and the diagnostic names none of what survived.
func TestRm_continuesToRemainingOperands_whenOneFails(t *testing.T) {
	// Given
	directory := t.TempDir()
	first := filepath.Join(directory, "first")
	last := filepath.Join(directory, "last")
	missing := filepath.Join(directory, "missing")
	for _, name := range []string{first, last} {
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// When
	_, stderr, err := runApplet(t, "rm", first, missing, last)

	// Then: the operand after the failure is the one that matters here.
	if _, statErr := os.Lstat(last); !os.IsNotExist(statErr) {
		t.Fatalf("the operand after the failing one survived (%v)", statErr)
	}
	if _, statErr := os.Lstat(first); !os.IsNotExist(statErr) {
		t.Fatalf("the operand before the failing one survived (%v)", statErr)
	}
	if err == nil {
		t.Fatal("expected a non-zero status")
	}
	if status, ok := applets.StatusCode(err); !ok || status != 1 {
		t.Fatalf("status = %d, recognised = %v; want 1, true", status, ok)
	}
	if !strings.Contains(stderr, "missing") {
		t.Fatalf("stderr = %q, want it to name the operand that failed", stderr)
	}
}

// Without -r a directory is refused, empty or not, and -f does not excuse it.
//
// os.Remove unlinks an empty directory quite happily, so `rm d` deleted
// something both busybox and POSIX refuse to touch -- a shell being more
// destructive than its own reference. Measured, busybox answers
// `rm: 'd' is a directory` and exits 1 for `rm d` and for `rm -f d` alike: the
// check sits before it looks at FILEUTILS_FORCE (libbb/remove_file.c:35).
func TestRm_refusesADirectory_withoutTheRecursiveFlag(t *testing.T) {
	for _, forced := range []bool{false, true} {
		name := "rm"
		args := []string{}
		if forced {
			name, args = "rm -f", []string{"-f"}
		}
		t.Run(name, func(t *testing.T) {
			// Given
			directory := t.TempDir()
			target := filepath.Join(directory, "d")
			if err := os.Mkdir(target, 0o755); err != nil {
				t.Fatal(err)
			}

			// When
			_, stderr, err := runApplet(t, "rm", append(args, target)...)

			// Then
			if err == nil {
				t.Fatal("expected a failure")
			}
			if status, ok := applets.StatusCode(err); !ok || status != 1 {
				t.Fatalf("status = %d, recognised = %v; want 1, true", status, ok)
			}
			if want := "rm: '" + target + "' is a directory\n"; stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			if _, statErr := os.Lstat(target); statErr != nil {
				t.Fatalf("the directory was removed anyway (%v)", statErr)
			}
		})
	}
}

// -f still swallows what was never there, and one absent operand must not
// invent a failure for the whole command.
func TestRm_forceIgnoresAbsentOperands_acrossTheWholeList(t *testing.T) {
	// Given
	directory := t.TempDir()
	present := filepath.Join(directory, "present")
	if err := os.WriteFile(present, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// When
	_, stderr, err := runApplet(t, "rm", "-f",
		filepath.Join(directory, "absent-one"), present, filepath.Join(directory, "absent-two"))

	// Then
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if _, statErr := os.Lstat(present); !os.IsNotExist(statErr) {
		t.Fatalf("the operand between two absent ones survived (%v)", statErr)
	}
}
