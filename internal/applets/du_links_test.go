package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// du counts what it meets once, as busybox's does: a directory named twice, and a file by each
// of its links, unless -l. It counted a hard-linked file by every name, and the same
// directory as often as it was named, where busybox-w32 prints it once.
func TestDu_countsWhatItMeetsOnce(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	if err := os.MkdirAll(filepath.Join(dir, "t", "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "t", "a", "f"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "t", "one"), []byte("12345678"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(dir, "t", "one"), filepath.Join(dir, "t", "two")); err != nil {
		t.Fatal(err)
	}
	named := func(args ...string) string {
		t.Helper()
		stdout, stderr, err := runPermuted(t, view, "", append([]string{"du", "-b"}, args...)...)
		if err != nil || stderr != "" {
			t.Fatalf("du -b %q: %q, %v", args, stderr, err)
		}
		var names []string
		for _, line := range strings.Split(strings.TrimSuffix(stdout, "\n"), "\n") {
			_, name, _ := strings.Cut(line, "\t")
			names = append(names, name)
		}
		return strings.Join(names, " ")
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"t/a", "t/a"}, "t/a"},
		{[]string{"-l", "t/a", "t/a"}, "t/a t/a"},
		{[]string{"t/one", "t/two"}, "t/one"},
		{[]string{"-l", "t/one", "t/two"}, "t/one t/two"},
		// A file with one link is its own every time it is named.
		{[]string{"t/a/f", "t/a/f"}, "t/a/f t/a/f"},
	} {
		if got := named(test.args...); got != test.want {
			t.Errorf("du -b %q named %q, want %q", test.args, got, test.want)
		}
	}
	stdout, _, err := runPermuted(t, view, "", "du", "-b", "-s", "-c", "t/one", "t/two")
	if err != nil || !strings.HasSuffix(stdout, "8\ttotal\n") {
		t.Errorf("du -b -s -c t/one t/two: %q, %v; want the linked file counted once in the total", stdout, err)
	}
}

// -L follows every link and -H only one named as FILE, and a directory reached by a link after
// it was met under its own name is not counted again, which also keeps a loop of links from
// being walked for ever. A link that is not followed is counted as itself: the length of what
// it holds, as busybox-w32's stat gives it on Windows and every stat gives it elsewhere.
func TestDu_followsLinksAsAsked(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	if err := os.MkdirAll(filepath.Join(dir, "t", "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "t", "a", "f"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a", filepath.Join(dir, "t", "link")); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	// A loop: t/a/back is t/a's own parent.
	if err := os.Symlink("..", filepath.Join(dir, "t", "a", "back")); err != nil {
		t.Fatal(err)
	}
	lines := func(args ...string) []string {
		t.Helper()
		stdout, stderr, err := runPermuted(t, view, "", append([]string{"du"}, args...)...)
		if err != nil || stderr != "" {
			t.Fatalf("du %q: %q, %v", args, stderr, err)
		}
		return strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	}
	names := func(found []string) string {
		var names []string
		for _, line := range found {
			_, name, _ := strings.Cut(line, "\t")
			names = append(names, name)
		}
		return strings.Join(sorted(names), " ")
	}
	if got := names(lines("-a", "t")); got != "t t/a t/a/back t/a/f t/link" {
		t.Errorf("du -a t named %q, want each link as itself", got)
	}
	if got := names(lines("-a", "-b", "t/link")); got != "t/link" {
		t.Errorf("du -a -b t/link named %q, want the link alone", got)
	}
	if got := lines("-b", "t/link"); got[0] != "1\tt/link" {
		t.Errorf("du -b t/link printed %q, want the length of `a`", got)
	}
	if got := names(lines("-a", "-H", "t")); got != "t t/a t/a/back t/a/f t/link" {
		t.Errorf("du -a -H t named %q, want no link followed, none being named", got)
	}
	// Past the link it followed, -H follows every one, as busybox converts it to -L: back is
	// t, whose a has been met already as t/link.
	if got := names(lines("-a", "-H", "t/link")); got != "t/link t/link/back t/link/f" {
		t.Errorf("du -a -H t/link named %q, want what it points at", got)
	}
	// Under -L the loop through back and the second way into a both end where they begin.
	if got := names(lines("-a", "-L", "t")); got != "t t/a t/a/f" {
		t.Errorf("du -a -L t named %q, want t/a once and the loop not walked", got)
	}
	// -l counts a by both ways into it, and the loop still ends: back leads into t, which is
	// being walked.
	if got := names(lines("-a", "-L", "-l", "t")); got != "t t/a t/a/f t/link t/link/f" {
		t.Errorf("du -a -L -l t named %q, want t/a by both ways in and the loop not walked", got)
	}
}
