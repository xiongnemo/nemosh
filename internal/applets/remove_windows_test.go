//go:build windows

package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// holdOpenWithoutDeleteSharing makes a file genuinely unremovable for as long as
// the test runs.
//
// This is the ordinary Windows case, not a contrived one: a file open in another
// process without FILE_SHARE_DELETE cannot be unlinked, which is why `rm -rf`
// on a build directory fails when something is still running. os.Open would not
// do -- Go asks for delete sharing, so a file it opened can still be removed.
func holdOpenWithoutDeleteSharing(t *testing.T, path string) {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("holding %s open: %v", path, err)
	}
	t.Cleanup(func() { windows.CloseHandle(handle) })
}

// What a diagnostic has to answer here is "which file", and naming the operand
// does not answer it. Measured, busybox says `can't remove 'b/held.exe'` where
// this said `cannot remove 'b'` -- and on a tree the size of node_modules the
// operand name is exactly the thing the reader already knows.
//
// The second half is the absence of noise. A directory that still holds
// something is a consequence of the failure already reported, not a new one, so
// busybox does not report it: two held files under one tree produce two lines
// there, not five. That comes from `if (status == 0 && rmdir(path) < 0)` in
// libbb/remove_file.c -- the parent is never attempted once a child has failed.
func TestRm_namesTheFileInUse_andStillRemovesEverythingElse(t *testing.T) {
	// Given
	directory := t.TempDir()
	tree := filepath.Join(directory, "tree")
	held := filepath.Join(tree, "locked", "held.bin")
	freed := filepath.Join(tree, "free", "spare.bin")
	other := filepath.Join(directory, "other")
	sibling := filepath.Join(other, "g.bin")
	for _, file := range []string{held, freed, sibling} {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	holdOpenWithoutDeleteSharing(t, held)

	// When
	_, stderr, err := runApplet(t, "rm", "-rf", tree, other)

	// Then
	if err == nil {
		t.Fatal("expected a non-zero status")
	}
	if _, statErr := os.Lstat(held); statErr != nil {
		t.Fatalf("the held file was removed after all (%v)", statErr)
	}
	if _, statErr := os.Lstat(freed); !os.IsNotExist(statErr) {
		t.Fatalf("a removable file beside the held one survived (%v)", statErr)
	}
	if _, statErr := os.Lstat(other); !os.IsNotExist(statErr) {
		t.Fatalf("the operand after the failing one survived (%v)", statErr)
	}

	// And: one line, naming the file rather than the operand.
	if !strings.Contains(stderr, "held.bin") {
		t.Fatalf("stderr = %q, want it to name the file that is in use", stderr)
	}
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	if len(lines) != 1 {
		t.Fatalf("stderr had %d lines, want 1 -- a directory left non-empty is a consequence, not a second failure:\n%s", len(lines), stderr)
	}
}
