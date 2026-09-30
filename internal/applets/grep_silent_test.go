package applets_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// grep -s says nothing of a FILE it cannot open and still answers 2, as busybox's does: -s
// silences the message, not the failure. It answered 0 when another FILE matched, and 1 when
// the missing FILE was the only one. -q still answers 0 at a match. Each answer is
// busybox-w32's, measured.
func TestGrep_silentStillAnswersTwoForAMissingFile(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	if err := os.WriteFile(filepath.Join(dir, "gs"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args   []string
		stdout string
		status int
	}{
		{[]string{"grep", "-s", "o", "nosuch", "gs"}, "gs:one\ngs:two\n", 2},
		{[]string{"grep", "-s", "o", "nosuch"}, "", 2},
		{[]string{"grep", "-s", "zzz", "nosuch", "gs"}, "", 2},
		{[]string{"grep", "-sc", "o", "nosuch", "gs"}, "gs:2\n", 2},
		{[]string{"grep", "-qs", "o", "nosuch", "gs"}, "", 0},
	} {
		stdout, stderr, err := runPermuted(t, view, "", test.args...)
		status := 0
		if err != nil {
			status = 1
			if code, ok := applets.StatusCode(err); ok {
				status = code
			}
		}
		if stdout != test.stdout || stderr != "" || status != test.status {
			t.Errorf("%q: got %q, %q, %d; want %q and %d, nothing said", test.args, stdout, stderr, status, test.stdout, test.status)
		}
	}
}
