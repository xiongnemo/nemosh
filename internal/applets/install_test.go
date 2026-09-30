package applets_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// install is busybox's: it copies, makes directories with -d and -D, copies into -t's DIR or a
// DEST that is a directory, names each copy with -v, and gives each 0755 or -m's MODE. There was
// no install at all.
func TestInstall_copiesAndSetsAttributesAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	for name, text := range map[string]string{"a": "hi\n", "b": "there\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args       []string
		wantStdout string
		want       map[string]string
	}{
		{[]string{"install", "a", "a2"}, "", map[string]string{"a2": "hi\n"}},
		{[]string{"install", "-v", "a", "a3"}, "'a' -> 'a3'\n", map[string]string{"a3": "hi\n"}},
		{[]string{"install", "a", "b", "d"}, "", map[string]string{"d/a": "hi\n", "d/b": "there\n"}},
		{[]string{"install", "-D", "a", "p/q/r"}, "", map[string]string{"p/q/r": "hi\n"}},
		{[]string{"install", "-D", "-t", "s/t", "a", "b"}, "", map[string]string{"s/t/a": "hi\n", "s/t/b": "there\n"}},
		{[]string{"install", "--target-directory=d", "b"}, "", map[string]string{"d/b": "there\n"}},
		{[]string{"install", "-d", "-v", "n1/n2"}, "created directory: 'n1/'\ncreated directory: 'n1/n2'\n", nil},
		{[]string{"install", "-c", "-b", "a", "a4"}, "", map[string]string{"a4": "hi\n"}},
	} {
		stdout, stderr, err := runPermuted(t, view, "", test.args...)
		if err != nil || stdout != test.wantStdout {
			t.Errorf("%q: got %q, %q, %v; want %q", test.args, stdout, stderr, err, test.wantStdout)
		}
		for name, text := range test.want {
			if got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name))); err != nil || string(got) != text {
				t.Errorf("%q: %s holds %q, %v; want %q", test.args, name, got, err, text)
			}
		}
	}
	if info, err := os.Stat(filepath.Join(dir, "n1", "n2")); err != nil || !info.IsDir() {
		t.Errorf("install -d n1/n2 made no directory: %v", err)
	}
	// The mode is 0755 with no -m, whatever the umask, and -m's otherwise. Windows keeps only
	// the owner's write bit.
	if runtime.GOOS != "windows" {
		if _, _, err := runPermuted(t, view, "", "install", "-m", "640", "a", "a5"); err != nil {
			t.Fatal(err)
		}
		for name, want := range map[string]os.FileMode{"a2": 0o755, "a5": 0o640, "n1/n2": 0o755} {
			if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err != nil || info.Mode().Perm() != want {
				t.Errorf("%s: mode %v, %v; want %v", name, info.Mode().Perm(), err, want)
			}
		}
	}
}

// What busybox's install refuses, and the owner, the group, -p and -s. -s runs strip, which is a
// program rather than an applet, so it is not found, as busybox says when it has none.
func TestInstall_refusesAndReportsAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	source := filepath.Join(dir, "a")
	if err := os.WriteFile(source, []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(source, past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args    []string
		wantErr string
		stderr  string
	}{
		{[]string{"install", "a"}, "missing operand", ""},
		{[]string{"install", "-d", "-t", "d", "x"}, "-d cannot be given with -t or -s", ""},
		{[]string{"install", "-m", "bogus", "a", "m"}, "invalid mode 'bogus'", ""},
		{[]string{"install", "-o", "no-such-user-here", "a", "o"}, "unknown user no-such-user-here", ""},
		{[]string{"install", "-g", "no-such-group-here", "a", "g"}, "unknown group no-such-group-here", ""},
		{[]string{"install", "nosuch", "x"}, "exit status 1", "install: cannot stat 'nosuch': No such file or directory\n"},
		{[]string{"install", "d", "x"}, "exit status 1", "install: omitting directory 'd'\n"},
		{[]string{"install", "-s", "a", "s"}, "exit status 1", "install: strip: No such file or directory\n"},
	} {
		_, stderr, err := runPermuted(t, view, "", test.args...)
		if err == nil || !strings.Contains(err.Error(), test.wantErr) || stderr != test.stderr {
			t.Errorf("%q: got %q, %v; want %q, %q", test.args, stderr, err, test.stderr, test.wantErr)
		}
	}
	for _, name := range []string{"m", "o", "g", "x"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("a refused install made %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "s")); err != nil {
		t.Errorf("install -s made no copy before it failed to strip: %v", err)
	}
	// The session's own account is a user and a group everywhere, and -p keeps the times.
	me := applets.CurrentUserName()
	if _, stderr, err := runPermuted(t, view, "", "install", "-p", "-o", me, "-g", me, "a", "p"); err != nil {
		t.Fatalf("install -o %s -g %s: %q, %v", me, me, stderr, err)
	}
	if info, err := os.Stat(filepath.Join(dir, "p")); err != nil || !info.ModTime().Equal(past) {
		t.Errorf("install -p: the copy's time is %v, %v; want %v", info.ModTime(), err, past)
	}
}
