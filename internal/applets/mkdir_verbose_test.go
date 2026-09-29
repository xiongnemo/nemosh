package applets_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// mkdir -v and rmdir -v say what they make and remove, as busybox's do, and rmdir
// --ignore-fail-on-non-empty passes over a directory that is not empty. -v said nothing, and the
// long option was refused.
func TestMkdirAndRmdir_sayWhatTheyDoAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	for _, test := range []struct {
		args       []string
		wantStdout string
	}{
		{[]string{"mkdir", "-v", "a"}, "created directory: 'a'\n"},
		{[]string{"mkdir", "-pv", "b/c/d"}, "created directory: 'b/'\ncreated directory: 'b/c/'\ncreated directory: 'b/c/d'\n"},
		{[]string{"mkdir", "-pv", "b/c/e"}, "created directory: 'b/c/e'\n"},
		{[]string{"mkdir", "--parents", "--verbose", "a"}, ""},
		{[]string{"rmdir", "-v", "a"}, "rmdir: removing directory, 'a'\n"},
		{[]string{"rmdir", "-pv", "b/c/d"}, "rmdir: removing directory, 'b/c/d'\nrmdir: removing directory, 'b/c'\n"},
	} {
		stdout, _, _ := runPermuted(t, view, "", test.args...)
		if stdout != test.wantStdout {
			t.Errorf("%q: got %q, want %q", test.args, stdout, test.wantStdout)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "b", "c", "e", "held"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := runPermuted(t, view, "", "rmdir", "--ignore-fail-on-non-empty", "b/c/e"); err != nil || stderr != "" {
		t.Errorf("rmdir --ignore-fail-on-non-empty on a full directory: %q, %v; want it passed over", stderr, err)
	}
	if _, _, err := runPermuted(t, view, "", "rmdir", "b/c/e"); err == nil {
		t.Error("rmdir of a full directory succeeded")
	}
}

// A failure names the step of -p it came at and what stat says of it, as busybox's does: it
// named the whole operand, and `File exists`. A drive's root, which Windows refuses to make with
// ERROR_ACCESS_DENIED, is there as busybox-w32 has it, so `mkdir -p /c/x` makes x.
func TestMkdir_namesTheStepThatFailedAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	if err := os.WriteFile(filepath.Join(dir, "f"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	root := filepath.VolumeName(dir) + "/"
	for _, test := range []struct {
		args    []string
		wantErr string
	}{
		{[]string{"mkdir", "-p", "f/x"}, "cannot create directory 'f/': Not a directory"},
		{[]string{"mkdir", "-p", "f/"}, "cannot create directory 'f/': Not a directory"},
		{[]string{"mkdir", "-p", "f"}, "cannot create directory 'f': File exists"},
		{[]string{"mkdir", "."}, ""},
		{[]string{"mkdir", "-p", root}, ""},
	} {
		_, _, err := runPermuted(t, view, "", test.args...)
		if (err == nil) != (test.wantErr == "") || err != nil && err.Error() != test.wantErr {
			t.Errorf("%q: got %v, want %q", test.args, err, test.wantErr)
		}
	}
	if runtime.GOOS == "windows" {
		if _, _, err := runPermuted(t, view, "", "mkdir", root); err == nil || err.Error() != "cannot create directory '"+root+"': File exists" {
			t.Errorf("mkdir %s: got %v, want File exists", root, err)
		}
	}
}
