package applets_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// patch does what busybox's does but for where a hunk may land: `patching file F` on stdout,
// `creating F` from a /dev/null old side, parent directories and all, an emptied file from a
// /dev/null new side and with -E `removing F`, the +++ name, the basename without -p, ORIGFILE
// and PATCHFILE operands, --dry-run and the long options, `\ No newline at end of file` either
// way, and a file whose hunk fails left as it was while the next is patched. Each answer is
// busybox-w32's, measured.
func TestPatch_isBusyboxs(t *testing.T) {
	const change = "@@ -1 +1 @@\n-a\n+b\n"
	for _, test := range []struct {
		name          string
		files         map[string]string
		args          []string
		patch         string
		stdout        string
		status        int
		want, missing []string
	}{
		{name: "creating", args: []string{"-p0"}, patch: "--- /dev/null\n+++ sub/dir/new\n@@ -0,0 +1,2 @@\n+one\n+two\n",
			stdout: "creating sub/dir/new\n", want: []string{"sub/dir/new", "one\ntwo\n"}},
		{name: "creating, dated as diff -N dates it", patch: "--- new\t1970-01-01 00:00:00.000000000 +0000\n+++ new\t2024-05-06 07:08:09.000000000 +0000\n@@ -0,0 +1 @@\n+two\n",
			stdout: "creating new\n", want: []string{"new", "two\n"}},
		{name: "emptying", files: map[string]string{"old": "gone\n"}, patch: "--- old\n+++ /dev/null\n@@ -1 +0,0 @@\n-gone\n",
			stdout: "patching file old\n", want: []string{"old", ""}},
		{name: "removing", files: map[string]string{"old": "gone\n"}, args: []string{"-E"}, patch: "--- old\n+++ /dev/null\n@@ -1 +0,0 @@\n-gone\n",
			stdout: "removing old\n", missing: []string{"old"}},
		{name: "the basename without -p", files: map[string]string{"f": "a\n"}, patch: "--- x/y/f\n+++ x/y/f\n" + change,
			stdout: "patching file f\n", want: []string{"f", "b\n"}},
		{name: "-p1", files: map[string]string{"d/f": "a\n"}, args: []string{"-p1"}, patch: "--- a/d/f\n+++ b/d/f\n" + change,
			stdout: "patching file d/f\n", want: []string{"d/f", "b\n"}},
		{name: "the +++ name", files: map[string]string{"new": "a\n", "old": "a\n"}, patch: "--- old\n+++ new\n" + change,
			stdout: "patching file new\n", want: []string{"new", "b\n", "old", "a\n"}},
		{name: "ORIGFILE and PATCHFILE", files: map[string]string{"of": "a\n", "p": "--- x\n+++ x\n" + change}, args: []string{"of", "p"},
			stdout: "patching file of\n", want: []string{"of", "b\n"}},
		{name: "--dry-run", files: map[string]string{"dr": "a\n"}, args: []string{"--dry-run"}, patch: "--- dr\n+++ dr\n" + change,
			stdout: "patching file dr\n", want: []string{"dr", "a\n"}},
		{name: "long options", files: map[string]string{"lo": "a\n", "p": "--- lo\n+++ lo\n" + change}, args: []string{"--strip=0", "--input=p", "-u"},
			stdout: "patching file lo\n", want: []string{"lo", "b\n"}},
		{name: "a newline added", files: map[string]string{"nn": "a"}, patch: "--- nn\n+++ nn\n@@ -1 +1,2 @@\n-a\n\\ No newline at end of file\n+a\n+b\n",
			stdout: "patching file nn\n", want: []string{"nn", "a\nb\n"}},
		{name: "a newline taken away", files: map[string]string{"mm": "a\n"}, patch: "--- mm\n+++ mm\n" + change + "\\ No newline at end of file\n",
			stdout: "patching file mm\n", want: []string{"mm", "b"}},
		{name: "a failed file and the next", files: map[string]string{"u1": "z\n", "u2": "a\n"},
			patch:  "--- u1\n+++ u1\n" + change + "--- u2\n+++ u2\n" + change,
			stdout: "patching file u1\npatching file u2\n", status: 1, want: []string{"u1", "z\n", "u2", "b\n"}},
		{name: "a removal that reads like a header", files: map[string]string{"h": "-- x\n"}, patch: "--- h\n+++ h\n@@ -1 +1 @@\n--- x\n+y\n",
			stdout: "patching file h\n", want: []string{"h", "y\n"}},
		{name: "nothing to apply", patch: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, text := range test.files {
				path := filepath.Join(dir, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			stdout, stderr, err := runSmall(t, dir, test.patch, "patch", test.args...)
			if status, _ := applets.StatusCode(err); stdout != test.stdout || status != test.status {
				t.Errorf("patch %q = %q, status %d (%v, %q); want %q, status %d", test.args, stdout, status, err, stderr, test.stdout, test.status)
			}
			for index := 0; index+1 < len(test.want); index += 2 {
				data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(test.want[index])))
				if err != nil || string(data) != test.want[index+1] {
					t.Errorf("%s = %q, %v; want %q", test.want[index], data, err, test.want[index+1])
				}
			}
			for _, name := range test.missing {
				if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
					t.Errorf("%s is still there", name)
				}
			}
		})
	}
	for _, test := range []struct {
		files map[string]string
		args  []string
		patch string
		want  string
	}{
		{map[string]string{"new": "x\n"}, nil, "--- /dev/null\n+++ new\n@@ -0,0 +1 @@\n+one\n", "cannot open 'new': File exists"},
		{nil, []string{"-i", "nosuch"}, "", "cannot open 'nosuch': No such file or directory"},
		{nil, []string{"-l"}, "", "invalid option -- 'l'"},
		{nil, []string{"-p", "x"}, "", "invalid number 'x'"},
	} {
		dir := writeSmallFixture(t, test.files)
		if _, _, err := runSmall(t, dir, test.patch, "patch", test.args...); err == nil || err.Error() != test.want {
			t.Errorf("patch %q: %v, want %q", test.args, err, test.want)
		}
	}
}
