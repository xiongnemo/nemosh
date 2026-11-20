package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// realpath takes GNU's options: --relative-to and --relative-base, -m for a path that need not
// be there, -e, -s, -q and -z. busybox's took none, so `realpath --relative-to=. x` looked for a
// file called --relative-to=.
func TestRealpath_gnuOptions(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "b", "c"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	absolute := func(path string) string {
		stdout, _, err := runAppletWithInput(t, "", "realpath", path)
		if err != nil {
			t.Fatalf("realpath %s: %v", path, err)
		}
		return strings.TrimSuffix(stdout, "\n")
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"--relative-to=a", "a/b/c"}, want: "b/c\n"},
		{args: []string{"--relative-to", "a/b", "a"}, want: "..\n"},
		{args: []string{"--relative-to=a/b/c", "a/b/c"}, want: ".\n"},
		{args: []string{"--relative-base=a", "a/b/c"}, want: "b/c\n"},
		// Not under the base: absolute, as GNU's prints it.
		{args: []string{"--relative-base=a/b", "a"}, want: absolute("a") + "\n"},
		{args: []string{"-m", "a/nosuch/deeper"}, want: absolute("a") + "/nosuch/deeper\n"},
		{args: []string{"--canonicalize-missing", "x/y"}, want: filepath.ToSlash(filepath.Dir(absolute("a"))) + "/x/y\n"},
		{args: []string{"-z", "a"}, want: absolute("a") + "\x00"},
		// -s resolves nothing, so the directory is as the working directory spells it: on a runner that
		// is the 8.3 RUNNER~1 the temporary directory is given as, where the others answer runneradmin.
		{args: []string{"-s", "a/./b/../b"}, want: filepath.ToSlash(dir) + "/a/b\n"},
		{args: []string{"--", "a"}, want: absolute("a") + "\n"},
	} {
		stdout, stderr, err := runAppletWithInput(t, "", "realpath", test.args...)
		if err != nil || stdout != test.want {
			t.Errorf("realpath %q = %q, %v (%s); want %q", test.args, stdout, err, stderr, test.want)
		}
	}
	// -q says nothing of what is not there, and still fails; -e fails where -m would not.
	if stdout, stderr, err := runAppletWithInput(t, "", "realpath", "-q", "nosuch"); err == nil || stdout != "" || stderr != "" {
		t.Errorf("realpath -q nosuch = %q, %q, %v", stdout, stderr, err)
	}
	if _, _, err := runAppletWithInput(t, "", "realpath", "-e", "a/nosuch"); err == nil {
		t.Error("realpath -e a/nosuch succeeded")
	}
	if _, stderr, err := runAppletWithInput(t, "", "realpath", "-Z", "a"); err == nil || !strings.Contains(stderr+errorText(err), "unknown option -- Z") {
		t.Errorf("realpath -Z: %v, %s", err, stderr)
	}
}
