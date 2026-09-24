//go:build windows

package runtime_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// busybox-w32's two options, measured there on the same four files: nohiddenglob leaves
// out anything Hidden, nohidsysglob only what is Hidden and System, and a pattern that
// then matches nothing is left as written.
func TestGlob_leavesOutHiddenFilesWhenAsked(t *testing.T) {
	dir := t.TempDir()
	for name, attributes := range map[string]uint32{
		"plain":  0,
		"hid":    syscall.FILE_ATTRIBUTE_HIDDEN,
		"sys":    syscall.FILE_ATTRIBUTE_SYSTEM,
		"hidsys": syscall.FILE_ATTRIBUTE_HIDDEN | syscall.FILE_ATTRIBUTE_SYSTEM,
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if attributes == 0 {
			continue
		}
		pointer, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := syscall.SetFileAttributes(pointer, attributes); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct{ options, pattern, want string }{
		{options: "", pattern: "*", want: "hid hidsys plain sys"},
		{options: "set -o nohiddenglob;", pattern: "*", want: "plain sys"},
		{options: "set -o nohidsysglob;", pattern: "*", want: "hid plain sys"},
		{options: "set -o nohiddenglob;", pattern: "hid*", want: "hid*"},
	} {
		script := test.options + " cd '" + filepath.ToSlash(dir) + "' && echo " + test.pattern
		if stdout, _ := runScriptCapturing(script); stdout != test.want+"\n" {
			t.Errorf("%q = %q, want %q", script, stdout, test.want)
		}
	}
}
