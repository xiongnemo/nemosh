package applets_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// An empty operand names nothing, as POSIX resolves no null pathname: each applet fails it with
// ENOENT, as busybox's do, and the working directory is left as it was. The shell's view joined
// it to the working directory, so `rm -rf ""` emptied that, and touch, chmod, ls and du used it.
func TestApplets_anEmptyOperandNamesNothing(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	if err := os.WriteFile(filepath.Join(dir, "keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"rm", "-r", ""},
		{"rmdir", ""},
		{"mkdir", ""},
		{"mkdir", "-p", ""},
		{"touch", ""},
		{"chmod", "644", ""},
		{"ls", ""},
		{"ls", "-d", ""},
		{"du", ""},
		{"cat", ""},
		{"wc", ""},
		{"head", ""},
		{"cp", "keep", ""},
		{"cp", "", "copy"},
		{"cp", "-r", "", "copy"},
		{"mv", "keep", ""},
		{"ln", "keep", ""},
		{"ln", "-s", "keep", ""},
		{"find", ""},
		{"grep", "-r", "x", ""},
	} {
		_, stderr, err := runPermuted(t, view, "", args...)
		said := stderr
		if err != nil {
			said += err.Error()
		}
		if err == nil || !strings.Contains(said, "No such file or directory") {
			t.Errorf("%q: got %q, %v; want No such file or directory", args, stderr, err)
		}
	}
	for _, args := range [][]string{{"rm", "-rf", ""}, {"rm", "-f", ""}} {
		if _, stderr, err := runPermuted(t, view, "", args...); err != nil || stderr != "" {
			t.Errorf("%q: got %q, %v; want nothing said", args, stderr, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !slices.Equal(names, []string{"keep", "sub"}) {
		t.Errorf("the working directory holds %q, want keep and sub untouched", names)
	}
	// readlink -f and realpath answer the working directory, as busybox-w32's do.
	for _, name := range []string{"realpath", "readlink"} {
		args := []string{name, ""}
		if name == "readlink" {
			args = []string{name, "-f", ""}
		}
		got, _, err := runPermuted(t, view, "", args...)
		want, _, _ := runPermuted(t, view, "", append(args[:len(args)-1:len(args)-1], ".")...)
		if err != nil || got != want || got == "" {
			t.Errorf("%q: got %q, %v; want %q", args, got, err, want)
		}
	}
}
