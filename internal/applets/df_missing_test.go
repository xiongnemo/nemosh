package applets

import (
	"path/filepath"
	"strings"
	"testing"
)

// df of a FILE that is not there says so as busybox's does, whose find_mount_point stats it
// first, and fails; the heading is still printed, as busybox prints it. On Windows the row of
// the FILE's drive was answered, with status 0, and elsewhere it was "No such file or
// directory".
func TestDf_saysAFileThatIsNotThereHasNoMountPoint(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "nope")
	got, stderr, status := runApplet(t, "df", []string{missing}, "")
	if want := "df: " + missing + ": cannot find mount point\n"; stderr != want || status != 1 {
		t.Errorf("df %s = %q, status %d; want %q, status 1", missing, stderr, status, want)
	}
	if lines := strings.Split(strings.TrimRight(got, "\n"), "\n"); len(lines) != 1 || !strings.HasPrefix(lines[0], "Filesystem") {
		t.Errorf("df %s printed %q; want the heading alone", missing, got)
	}
}
