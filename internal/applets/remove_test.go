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
