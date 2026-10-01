package applets_test

import (
	"testing"
)

// Creating finds the names in -C's directory, wherever -C stands among the arguments, as
// busybox changes to it once the archive is open; the archive and -T's list are found where
// tar began. A -C that is not there is said in the C library's words. Each answer is
// busybox-w32's, measured, but for a -C that is a file, which busybox-w32 says is an Invalid
// argument and busybox elsewhere says is Not a directory.
func TestTar_createsInTheDirectoryOfMinusC(t *testing.T) {
	root := writeSmallFixture(t, map[string]string{"src/a.txt": "a\n", "src/sub/b.txt": "b\n", "list": "a.txt\n", "f": "x\n"})
	for _, test := range []struct {
		args   []string
		stored string
	}{
		{args: []string{"-C", "src", "-cf", "c.tar", "a.txt", "sub"}, stored: "a.txt\nsub/\nsub/b.txt\n"},
		{args: []string{"-cf", "c.tar", "a.txt", "-C", "src", "sub"}, stored: "a.txt\nsub/\nsub/b.txt\n"},
		{args: []string{"-C", "src", "-cf", "c.tar", "-T", "list"}, stored: "a.txt\n"},
	} {
		if _, stderr, err := runSmall(t, root, "", "tar", test.args...); err != nil {
			t.Fatalf("tar %q: %v (stderr %q)", test.args, err, stderr)
		}
		if stdout, _, err := runSmall(t, root, "", "tar", "tf", "c.tar"); stdout != test.stored || err != nil {
			t.Errorf("tar %q stored %q, %v; want %q", test.args, stdout, err, test.stored)
		}
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"-C", "nope", "-cf", "c.tar", "a.txt"}, want: "cannot change directory to 'nope': No such file or directory"},
		{args: []string{"-xf", "c.tar", "-C", "nope"}, want: "cannot change directory to 'nope': No such file or directory"},
		{args: []string{"-xf", "c.tar", "-C", "f"}, want: "cannot change directory to 'f': Not a directory"},
	} {
		if _, _, err := runSmall(t, root, "", "tar", test.args...); err == nil || err.Error() != test.want {
			t.Errorf("tar %q = %v; want %s", test.args, err, test.want)
		}
	}
}
