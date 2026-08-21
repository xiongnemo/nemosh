//go:build windows

package runtime

import (
	"strings"
	"testing"
	"time"
)

// The walkers over `/dev`.
//
// Stage 3 of docs/design/device-filesystem.md, and a measurement changed its shape. The plan assumed
// `find /` would meet `/dev` the way it does on Linux and that the walkers would therefore need a
// filesystem interface spanning both namespaces. They do not: `/` resolves to `/c` here, the current
// drive's root, so `/dev` is a *sibling* top-level name rather than a directory inside `/`. A walk of
// `/` cannot reach it, which removes both the splicing and the performance question.
//
// What remains is the explicit case -- `find /dev`, `du /dev`, `grep -r /dev` -- and one hazard that
// is real either way: reading a device while recursing. `grep -r /dev` over `/dev/zero` returns bytes
// for ever, so it must skip devices rather than read them, which is what GNU grep does when
// recursing and why it has that rule at all.

func TestDeviceWalk_findListsTheDevices(t *testing.T) {
	stdout, stderr, status := runScriptForDevices(t, "find /dev\n")
	if status != 0 {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
	// The directory itself first, as find prints the operand before its contents.
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if lines[0] != "/dev" {
		t.Fatalf("find /dev began with %q, want the operand itself", lines[0])
	}
	for _, want := range []string{"/dev/null", "/dev/zero", "/dev/clipboard", "/dev/stdout"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("find /dev = %q, missing %q", stdout, want)
		}
	}
}

// An expression narrows it, which is what makes find worth having over ls.
func TestDeviceWalk_findMatchesAnExpression(t *testing.T) {
	stdout, _, _ := runScriptForDevices(t, "find /dev -name null\n")
	if strings.TrimSpace(stdout) != "/dev/null" {
		t.Fatalf("find /dev -name null = %q", strings.TrimSpace(stdout))
	}
}

// `-type c` finds them, which is the predicate a device is for.
func TestDeviceWalk_findByType(t *testing.T) {
	stdout, _, _ := runScriptForDevices(t, "find /dev -type c\n")
	if !strings.Contains(stdout, "/dev/null") {
		t.Fatalf("find /dev -type c = %q, want the devices", stdout)
	}
	if strings.Contains(stdout, "\n/dev\n") || strings.TrimSpace(stdout) == "/dev" {
		t.Fatalf("find /dev -type c included the directory: %q", stdout)
	}
}

func TestDeviceWalk_duReportsZero(t *testing.T) {
	stdout, stderr, status := runScriptForDevices(t, "du -s /dev\n")
	if status != 0 {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
	if !strings.Contains(stdout, "/dev") {
		t.Fatalf("du -s /dev = %q", stdout)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout), "0") {
		t.Fatalf("du -s /dev = %q, want nothing counted for synthetic entries", strings.TrimSpace(stdout))
	}
}

// The hazard: recursing must not read a device. `/dev/zero` returns bytes for ever, so a grep that
// read it would never return -- which is why GNU grep skips devices when recursing.
func TestDeviceWalk_grepRecursiveSkipsDevices(t *testing.T) {
	done := make(chan struct{})
	var stdout, stderr string
	var status int
	go func() {
		stdout, stderr, status = runScriptForDevices(t, "grep -r anything /dev\n")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("grep -r /dev did not return: it is reading a device that never ends")
	}
	// Nothing matched, which is the honest answer for a set of files it declined to read.
	if status != 1 {
		t.Fatalf("grep -r /dev status %d, stderr %q, stdout %q", status, stderr, stdout)
	}
	if strings.Contains(stderr, "unsupported device") {
		t.Fatalf("grep -r /dev complained rather than skipping: %q", stderr)
	}
}

// A device named directly is still read, which is GNU's rule too: the skip is about recursing, not
// about devices being unreadable.
func TestDeviceWalk_grepReadsADeviceNamedDirectly(t *testing.T) {
	stdout, stderr, status := runScriptForDevices(t,
		"echo findme > /dev/clipboard && grep findme /dev/clipboard\n")
	if status != 0 {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
	if !strings.Contains(stdout, "findme") {
		t.Fatalf("grep of a named device = %q", stdout)
	}
}
