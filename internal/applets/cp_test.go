package applets_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func cpFixture(t *testing.T) (string, permuteTestView) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"src", "src/sub", "d"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{"f": "hello\n", "g": "old\n", "src/a": "a\n", "src/sub/b": "b\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, permuteTestView{cwd: dir}
}

func readFixture(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

// cp takes busybox's options, and prints and asks as busybox's does. It took -r and -R only, so
// `cp -rf`, `cp -a`, `cp -n` and the rest were refused as invalid options.
func TestCp_takesBusyboxsOptions(t *testing.T) {
	dir, view := cpFixture(t)
	for _, test := range []struct {
		args       []string
		stdin      string
		wantStdout string
		wantStderr string
	}{
		{[]string{"cp", "-v", "f", "g"}, "", "'f' -> 'g'\n", ""},
		{[]string{"cp", "-rf", "src", "r1"}, "", "", ""},
		{[]string{"cp", "-a", "src", "r2"}, "", "", ""},
		{[]string{"cp", "--recursive", "--verbose", "src/sub", "r3"}, "", "'src/sub/b' -> 'r3/b'\n'src/sub' -> 'r3'\n", ""},
		{[]string{"cp", "-v", "-t", "d", "f", "g"}, "", "'f' -> 'd/f'\n'g' -> 'd/g'\n", ""},
		{[]string{"cp", "--parents", "src/a", "d"}, "", "", ""},
	} {
		stdout, stderr, err := runPermuted(t, view, test.stdin, test.args...)
		if stdout != test.wantStdout || stderr != test.wantStderr || err != nil {
			t.Errorf("%q: got %q, %q, %v; want %q, %q", test.args, stdout, stderr, err, test.wantStdout, test.wantStderr)
		}
	}
	for _, path := range []string{"r1/a", "r2/sub/b", "r3/b", "d/f", "d/src/a"} {
		readFixture(t, filepath.Join(dir, path))
	}

	if err := os.WriteFile(filepath.Join(dir, "g"), []byte("kept\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args       []string
		stdin      string
		wantStderr string
		want       string
	}{
		{[]string{"cp", "-n", "f", "g"}, "", "", "kept\n"},
		{[]string{"cp", "-i", "f", "g"}, "n\n", "cp: overwrite 'g'? ", "kept\n"},
		{[]string{"cp", "-ni", "f", "g"}, "y\n", "cp: overwrite 'g'? ", "hello\n"},
	} {
		_, stderr, err := runPermuted(t, view, test.stdin, test.args...)
		if got := readFixture(t, filepath.Join(dir, "g")); stderr != test.wantStderr || err != nil || got != test.want {
			t.Errorf("%q: got %q, %v, g %q; want %q, g %q", test.args, stderr, err, got, test.wantStderr, test.want)
		}
	}
}

// -u copies only over an older file, and -p keeps the time a file was changed.
func TestCp_updatesAndPreserves(t *testing.T) {
	dir, view := cpFixture(t)
	past := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(filepath.Join(dir, "f"), past, past); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runPermuted(t, view, "", "cp", "-u", "f", "g"); err != nil || readFixture(t, filepath.Join(dir, "g")) != "old\n" {
		t.Errorf("cp -u over a newer file: %v; want it left alone", err)
	}
	if _, _, err := runPermuted(t, view, "", "cp", "-p", "f", "p"); err != nil {
		t.Fatalf("cp -p: %v", err)
	}
	if info, err := os.Stat(filepath.Join(dir, "p")); err != nil || !info.ModTime().Equal(past) {
		t.Errorf("cp -p: %v, time %v; want %v", err, info.ModTime(), past)
	}
}

// The failures busybox's cp names: a file copied onto itself, which cp emptied, a directory in
// the way of a file, a directory copied into itself, and -T with a directory for DEST.
func TestCp_refusesWhatBusyboxRefuses(t *testing.T) {
	dir, view := cpFixture(t)
	if err := os.Mkdir(filepath.Join(dir, "d", "f"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args       []string
		wantStderr string
	}{
		{[]string{"cp", "f", "f"}, "cp: 'f' and 'f' are the same file\n"},
		{[]string{"cp", "f", "d"}, "cp: cannot create 'd/f': Is a directory\n"},
		{[]string{"cp", "-r", "src", "src/inner"}, "cp: recursion detected, omitting directory 'src/inner'\n"},
		{[]string{"cp", "-T", "g", "d"}, "cp: 'd' is a directory\n"},
		{[]string{"cp", "src", "x"}, "cp: omitting directory 'src'\n"},
	} {
		_, stderr, err := runPermuted(t, view, "", test.args...)
		if err == nil {
			t.Errorf("%q succeeded; want it refused", test.args)
			continue
		}
		if got := stderr; got == "" {
			got = "cp: " + err.Error() + "\n"
			stderr = got
		}
		if stderr != test.wantStderr {
			t.Errorf("%q: got %q; want %q, as busybox says", test.args, stderr, test.wantStderr)
		}
	}
	if got := readFixture(t, filepath.Join(dir, "f")); got != "hello\n" {
		t.Errorf("cp f f left f holding %q", got)
	}
	if info, err := os.Stat(filepath.Join(dir, "d", "f")); err != nil || !info.IsDir() {
		t.Errorf("cp f d took away the directory d/f: %v", err)
	}
}

// A destination that is there is replaced, read-only or not, as busybox-w32 replaces it.
func TestCp_replacesAReadOnlyDestination(t *testing.T) {
	dir, view := cpFixture(t)
	target := filepath.Join(dir, "g")
	if err := os.Chmod(target, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(target, 0o644) })
	if _, stderr, err := runPermuted(t, view, "", "cp", "f", "g"); err != nil || stderr != "" {
		t.Fatalf("cp f g over a read-only g: %q, %v", stderr, err)
	}
	if got := readFixture(t, target); got != "hello\n" {
		t.Errorf("g holds %q, want the copy", got)
	}
}
