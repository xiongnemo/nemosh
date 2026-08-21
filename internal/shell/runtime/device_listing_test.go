package runtime

import (
	"strings"
	"testing"
)

// `/dev` as a directory: listable, globbable, and a directory to `test -d`.
//
// Stage 2 of docs/design/device-filesystem.md, and the one deliberate divergence from busybox-w32,
// which answers `ls /dev: No such file or directory`. Listing was chosen because it is the only
// thing that makes the devices discoverable: without it the way to learn what exists is to read
// documentation, and a shell that hides its own features behind a document has hidden them.

func TestDeviceListing_devIsADirectory(t *testing.T) {
	stdout, _, _ := runScriptForDevices(t, "test -d /dev && echo directory || echo no\n")
	if strings.TrimSpace(stdout) != "directory" {
		t.Fatalf("test -d /dev said %q, want directory", strings.TrimSpace(stdout))
	}
	// And not a file, so a script testing for one does not find the directory.
	stdout, _, _ = runScriptForDevices(t, "test -f /dev && echo file || echo no\n")
	if strings.TrimSpace(stdout) != "no" {
		t.Fatalf("test -f /dev said %q, want no", strings.TrimSpace(stdout))
	}
}

func TestDeviceListing_lsListsTheDevices(t *testing.T) {
	stdout, stderr, status := runScriptForDevices(t, "ls /dev\n")
	if status != 0 {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
	for _, want := range []string{"null", "zero", "random", "urandom", "clipboard"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("ls /dev = %q, missing %q", stdout, want)
		}
	}
	// The descriptor aliases are listed too: a person looking for what they can redirect to
	// wants to see stdout there, and it is a name this shell answers for.
	for _, want := range []string{"stdin", "stdout", "stderr"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("ls /dev = %q, missing the descriptor alias %q", stdout, want)
		}
	}
}

// The long form describes each one, which is what makes the listing worth having over a bare
// list of names.
func TestDeviceListing_lsLongListsThemAsDevices(t *testing.T) {
	stdout, stderr, status := runScriptForDevices(t, "ls -l /dev\n")
	if status != 0 {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) < 5 {
		t.Fatalf("ls -l /dev printed %d lines:\n%s", len(lines), stdout)
	}
	// The `total` header comes first, as it does for any directory. A directory the shell
	// provides is laid out by the same code as one on disk, because listing them differently
	// would suggest the entries were a different kind of thing than they are.
	if !strings.HasPrefix(lines[0], "total ") {
		t.Fatalf("ls -l /dev does not open with a total line: %q", lines[0])
	}
	for _, line := range lines[1:] {
		if !strings.HasPrefix(strings.TrimSpace(line), "c") {
			t.Fatalf("ls -l /dev has a line that is not a character device: %q", line)
		}
	}
}

// Globbing reaches them, which is the other half of discoverable: `echo /dev/*` is how somebody
// finds out what is there without knowing the applet to ask.
func TestDeviceListing_globExpandsUnderDev(t *testing.T) {
	stdout, stderr, status := runScriptForDevices(t, "echo /dev/*\n")
	if status != 0 {
		t.Fatalf("status %d, stderr %q", status, stderr)
	}
	for _, want := range []string{"/dev/null", "/dev/zero", "/dev/clipboard"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("echo /dev/* = %q, missing %q", strings.TrimSpace(stdout), want)
		}
	}
}

// A narrower pattern matches only what it should, so the glob is really matching rather than
// listing everything.
func TestDeviceListing_globMatchesAPattern(t *testing.T) {
	stdout, _, _ := runScriptForDevices(t, "echo /dev/*random\n")
	got := strings.Fields(strings.TrimSpace(stdout))
	if len(got) != 2 {
		t.Fatalf("echo /dev/*random = %v, want the two random devices", got)
	}
}

// A pattern that matches nothing stays literal, which is what a shell does with an unmatched
// glob and what tells the reader nothing was found.
func TestDeviceListing_unmatchedGlobStaysLiteral(t *testing.T) {
	stdout, _, _ := runScriptForDevices(t, "echo /dev/nosuch*\n")
	if strings.TrimSpace(stdout) != "/dev/nosuch*" {
		t.Fatalf("echo /dev/nosuch* = %q, want it unexpanded", strings.TrimSpace(stdout))
	}
}
