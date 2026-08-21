//go:build windows

package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A device has to be observable, not only openable.
//
// Stage 1 of docs/design/device-filesystem.md. Measured before this existed: `cat /dev/null` worked
// and `test -e /dev/null` said no, while `ls -l /dev/null` answered "is not a host path". Two
// divergences from busybox-w32, on paths this shell already opened correctly.
//
// The reference behaviour is busybox's *under its own shell*, which is worth stating because
// busybox is inconsistent about it: `busybox sh -c 'busybox ls -l /dev/null'` prints
// `crw-rw-rw- ... 0, 0 Jan 01 1970`, and the same `busybox ls -l /dev/null` invoked straight from
// another shell answers `ls: nul: No such file or directory`. Nemosh is a shell, so the first is the
// contract to match.

func runScriptForDevices(t *testing.T, script string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	status := rt.RunScript(context.Background(), script)
	return stdout.String(), stderr.String(), status
}

func TestDeviceStat_testSeesEveryDevice(t *testing.T) {
	// Every device the table carries, plus the descriptor aliases, which are paths a script
	// may reasonably ask about too.
	for _, path := range []string{
		"/dev/null", "/dev/zero", "/dev/random", "/dev/urandom", "/dev/clipboard",
	} {
		t.Run(path, func(t *testing.T) {
			stdout, stderr, status := runScriptForDevices(t,
				"test -e "+path+" && echo exists || echo missing\n")
			if status != 0 {
				t.Fatalf("status %d, stderr %q", status, stderr)
			}
			if strings.TrimSpace(stdout) != "exists" {
				t.Fatalf("test -e %s said %q, want exists", path, strings.TrimSpace(stdout))
			}
		})
	}
}

// A character device, which is what busybox reports and what Windows says of NUL.
func TestDeviceStat_isACharacterDevice(t *testing.T) {
	stdout, stderr, status := runScriptForDevices(t,
		"test -c /dev/null && echo chardev || echo no\n")
	if status != 0 {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
	if strings.TrimSpace(stdout) != "chardev" {
		t.Fatalf("test -c /dev/null said %q, want chardev", strings.TrimSpace(stdout))
	}
}

// And not a regular file or a directory, so a script testing for one does not find a device.
func TestDeviceStat_isNeitherFileNorDirectory(t *testing.T) {
	for predicate, want := range map[string]string{"-f": "no", "-d": "no", "-e": "yes"} {
		stdout, _, _ := runScriptForDevices(t,
			"test "+predicate+" /dev/null && echo yes || echo no\n")
		if strings.TrimSpace(stdout) != want {
			t.Fatalf("test %s /dev/null said %q, want %q", predicate, strings.TrimSpace(stdout), want)
		}
	}
}

// `ls -l` names it as a character device rather than refusing the path.
func TestDeviceStat_lsLongDescribesTheDevice(t *testing.T) {
	stdout, stderr, status := runScriptForDevices(t, "ls -l /dev/null\n")
	if status != 0 {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
	line := strings.TrimSpace(stdout)
	if !strings.HasPrefix(line, "c") {
		t.Fatalf("ls -l /dev/null = %q, want a mode beginning with c for a character device", line)
	}
	if !strings.Contains(line, "/dev/null") {
		t.Fatalf("ls -l /dev/null = %q, want the path named", line)
	}
}

// Reading and writing still work, which is the property Stage 1 must not break.
func TestDeviceStat_openingStillWorks(t *testing.T) {
	stdout, stderr, status := runScriptForDevices(t,
		"echo discarded > /dev/null && head -c 3 /dev/zero | wc -c\n")
	if status != 0 {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
	if strings.TrimSpace(stdout) != "3" {
		t.Fatalf("reading /dev/zero gave %q, want 3", strings.TrimSpace(stdout))
	}
}

// A name under /dev that is not a device is still absent, so the seam has not made the whole
// namespace exist.
func TestDeviceStat_anUnknownDeviceDoesNotExist(t *testing.T) {
	stdout, _, _ := runScriptForDevices(t,
		"test -e /dev/nosuchthing && echo exists || echo missing\n")
	if strings.TrimSpace(stdout) != "missing" {
		t.Fatalf("test -e /dev/nosuchthing said %q, want missing", strings.TrimSpace(stdout))
	}
}

// The whole line, against the reference.
//
// busybox-w32 under its own shell prints exactly this for /dev/null, and matching it to the column
// is the point of the stage: a character device, the current user in the owner column rather than
// the `root` a failed security lookup would give, major and minor numbers where a size would be,
// and the epoch for a modification time.
func TestDeviceStat_longListingMatchesTheReference(t *testing.T) {
	stdout, stderr, status := runScriptForDevices(t, "ls -l /dev/null\n")
	if status != 0 {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
	line := strings.TrimSpace(stdout)
	for _, want := range []string{"crw-rw-rw-", "0,   0", "Jan 01  1970", "/dev/null"} {
		if !strings.Contains(line, want) {
			t.Fatalf("ls -l /dev/null = %q, missing %q", line, want)
		}
	}
	// The owner column is the current account, not root. A failed security lookup gives root,
	// and root beside a device with the real user beside every file in the same listing reads
	// as a fault rather than a distinction.
	if strings.Contains(line, "root") {
		t.Fatalf("ls -l /dev/null = %q, want the current account rather than root", line)
	}
}

// A regular file keeps a size, so the device rule cannot have taken the size column from
// everything.
func TestDeviceStat_regularFilesKeepTheirSize(t *testing.T) {
	stdout, _, _ := runScriptForDevices(t, "ls -l go.mod\n")
	if strings.Contains(stdout, "0,   0") {
		t.Fatalf("ls -l go.mod = %q, want a size rather than device numbers", strings.TrimSpace(stdout))
	}
}
