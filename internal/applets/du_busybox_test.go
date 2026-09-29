package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// du walks as busybox's does: a directory after everything inside it, -a with the files, -s
// and -d as deep as asked, -c with a total, -b in bytes. The order its directories list their
// entries in is the filesystem's, so what is checked is that each name comes after all of its
// own. It printed a tree's directories in the reverse of the order it found them and took -s
// and -h alone.
func TestDu_walksAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	for _, path := range []string{"t/a/x", "t/b", "t/C"} {
		if err := os.MkdirAll(filepath.Join(dir, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "t", "a", "f"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "t", "b", "g"), make([]byte, 5000), 0o644); err != nil {
		t.Fatal(err)
	}
	// An empty file has no blocks anywhere; an empty directory has some on ext4.
	if err := os.WriteFile(filepath.Join(dir, "empty"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	names := func(args ...string) []string {
		t.Helper()
		stdout, stderr, err := runPermuted(t, view, "", append([]string{"du"}, args...)...)
		if err != nil || stderr != "" {
			t.Fatalf("du %q: %q, %v", args, stderr, err)
		}
		var found []string
		for _, line := range strings.Split(strings.TrimSuffix(stdout, "\n"), "\n") {
			_, name, _ := strings.Cut(line, "\t")
			found = append(found, name)
		}
		return found
	}
	postOrder := func(args []string, found []string, want ...string) {
		t.Helper()
		if strings.Join(sorted(found), " ") != strings.Join(sorted(want), " ") {
			t.Errorf("du %q named %q, want %q", args, found, want)
		}
		for index, name := range found {
			for _, later := range found[index+1:] {
				if strings.HasPrefix(later, name+"/") {
					t.Errorf("du %q printed %s before %s, which it holds", args, name, later)
				}
			}
		}
	}
	postOrder(nil, names("t"), "t/a/x", "t/a", "t/b", "t/C", "t")
	postOrder([]string{"-a"}, names("-a", "t"), "t/a/f", "t/a/x", "t/a", "t/b/g", "t/b", "t/C", "t")
	postOrder([]string{"-d1"}, names("-d1", "t"), "t/a", "t/b", "t/C", "t")
	if got := names("-s", "t"); strings.Join(got, " ") != "t" {
		t.Errorf("du -s t named %q, want t alone", got)
	}
	// Of -s and -d the later wins, as busybox's s-d:d-s has it.
	if got := names("-d", "2", "-s", "t"); strings.Join(got, " ") != "t" {
		t.Errorf("du -d 2 -s t named %q, want t alone", got)
	}
	if got := names("-s", "-d", "1", "t"); len(got) != 4 {
		t.Errorf("du -s -d 1 t named %q, want t and its three directories", got)
	}
	if got := names("-c", "-s", "t/a", "t/b"); strings.Join(got, " ") != "t/a t/b total" {
		t.Errorf("du -c -s t/a t/b named %q, want a total last", got)
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-b", "t/b/g"}, "5000\tt/b/g\n"},
		{[]string{"-bh", "t/b/g"}, "4.9K\tt/b/g\n"},
		{[]string{"-h", "empty"}, "0\tempty\n"},
	} {
		if stdout, _, err := runPermuted(t, view, "", append([]string{"du"}, test.args...)...); stdout != test.want || err != nil {
			t.Errorf("du %q: got %q, %v; want %q", test.args, stdout, err, test.want)
		}
	}
	stdout, stderr, err := runPermuted(t, view, "", "du", "-s", "missing", "t/C")
	if err == nil || stderr != "du: missing: No such file or directory\n" || !strings.HasSuffix(stdout, "\tt/C\n") {
		t.Errorf("du on a missing operand: %q, %q, %v; want it named and the rest measured", stdout, stderr, err)
	}
	if _, _, err := runPermuted(t, view, "", "du", "-d", "x", "t"); err == nil || !strings.Contains(err.Error(), "invalid number 'x'") {
		t.Errorf("du -d x: %v, want invalid number", err)
	}
}

func sorted(names []string) []string {
	copied := append([]string(nil), names...)
	for i := range copied {
		for j := i + 1; j < len(copied); j++ {
			if copied[j] < copied[i] {
				copied[i], copied[j] = copied[j], copied[i]
			}
		}
	}
	return copied
}
