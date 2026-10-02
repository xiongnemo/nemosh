package applets_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A name is stored as given and joined as busybox joins it, so `.` holds `./f.txt`. What busybox
// takes off a name -- a leading `/` or `../`, all up to the last `/../`, and here a drive -- is
// taken off and said once. Each answer is busybox-w32's, measured, but for the drive, which
// busybox-w32 keeps.
func TestTar_storesNamesAsBusyboxDoes(t *testing.T) {
	root := writeSmallFixture(t, map[string]string{"w/f.txt": "hi\n", "w/sub/g.txt": "g\n"})
	absolute := filepath.ToSlash(filepath.Join(root, "w", "f.txt"))
	prefix := "/"
	if runtime.GOOS == "windows" {
		prefix = absolute[:3]
	}
	for _, test := range []struct {
		dir            string
		args           []string
		stderr, stored string
	}{
		{dir: "w", args: []string{"cf", "../t.tar", "."}, stored: "./\n./f.txt\n./sub/\n./sub/g.txt\n"},
		{dir: "w/sub", args: []string{"cf", "../../t.tar", "../f.txt", "../f.txt"},
			stderr: "tar: removing leading '../' from member names\n", stored: "f.txt\nf.txt\n"},
		{dir: "w/sub", args: []string{"cf", "../../t.tar", "./../sub/../f.txt"},
			stderr: "tar: removing leading './../sub/../' from member names\n", stored: "f.txt\n"},
		{dir: "w", args: []string{"cf", "../t.tar", absolute},
			stderr: "tar: removing leading '" + prefix + "' from member names\n", stored: strings.TrimPrefix(absolute, prefix) + "\n"},
	} {
		// A view that takes an absolute name as it is, where runSmall's joins every name to its
		// directory.
		view := permuteTestView{cwd: filepath.Join(root, filepath.FromSlash(test.dir))}
		if _, stderr, err := runPermuted(t, view, "", append([]string{"tar"}, test.args...)...); err != nil || stderr != test.stderr {
			t.Errorf("tar %q = %q, %v; want %q", test.args, stderr, err, test.stderr)
		}
		if stdout, _, err := runSmall(t, root, "", "tar", "tf", "t.tar"); stdout != test.stored || err != nil {
			t.Errorf("tar %q stored %q, %v; want %q", test.args, stdout, err, test.stored)
		}
	}
}

// A name is looked for before an exclusion is asked about it, as busybox stats it first, so one
// that is not there is said even when it would have been left out.
func TestTar_saysAMissingNameItWouldExclude(t *testing.T) {
	root := writeSmallFixture(t, map[string]string{"f.txt": "hi\n"})
	_, stderr, err := runSmall(t, root, "", "tar", "cf", "t.tar", "--exclude", "nope", "nope", "f.txt")
	if status, _ := applets.StatusCode(err); status != 1 ||
		stderr != "tar: nope: No such file or directory\ntar: error exit delayed from previous errors\n" {
		t.Errorf("tar cf --exclude nope nope = %q, %v; want nope said and status 1", stderr, err)
	}
}
