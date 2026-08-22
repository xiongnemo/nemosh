package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// -t, -S, -r, -A, -d, -F and -R.
//
// `ls -ltr` is muscle memory, and every one of these was refused by name before
// 2026-08-22. Each expectation was measured against busybox-w32 v1.38.0 in a
// tree of the same shape on that date.

// lsSortFixture builds a tree whose name order, size order and time order are
// all different, so a test cannot pass by accident:
//
//	name:  a.txt  b.txt  c.txt  sub
//	size:  3      1      8      0
//	mtime: oldest middle newest (sub is newest of all)
func lsSortFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{"a.txt": "aaa", "b.txt": "b", "c.txt": "cccccccc"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "n.txt"), []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	for name, offset := range map[string]time.Duration{
		"a.txt": 0,
		"b.txt": 8 * 24 * time.Hour,
		"c.txt": 11 * 24 * time.Hour,
	} {
		when := base.Add(offset)
		if err := os.Chtimes(filepath.Join(dir, name), when, when); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func lsNames(t *testing.T, args ...string) []string {
	t.Helper()
	stdout, stderr, err := runAppletWithInput(t, "", "ls", args...)
	if err != nil {
		t.Fatalf("ls %v: %v (stderr %q)", args, err, stderr)
	}
	trimmed := strings.TrimSuffix(stdout, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func TestLs_sortOptions(t *testing.T) {
	dir := lsSortFixture(t)
	for _, test := range []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "default is by name",
			args: []string{"-1"},
			want: []string{"a.txt", "b.txt", "c.txt", "sub"},
		},
		{
			name: "-r reverses the name order",
			args: []string{"-1", "-r"},
			want: []string{"sub", "c.txt", "b.txt", "a.txt"},
		},
		{
			name: "-t is newest first",
			args: []string{"-1", "-t"},
			want: []string{"sub", "c.txt", "b.txt", "a.txt"},
		},
		{
			name: "-tr is oldest first",
			args: []string{"-1", "-t", "-r"},
			want: []string{"a.txt", "b.txt", "c.txt", "sub"},
		},
		{
			// Measured: busybox honours whichever sort arrived last, as GNU
			// documents. -S then -t sorts by time.
			name: "the last sort option wins, time last",
			args: []string{"-1", "-S", "-t"},
			want: []string{"sub", "c.txt", "b.txt", "a.txt"},
		},
		{
			name: "clustered with the long form",
			args: []string{"-1tr"},
			want: []string{"a.txt", "b.txt", "c.txt", "sub"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := lsNames(t, append(test.args, dir)...)
			if strings.Join(got, "|") != strings.Join(test.want, "|") {
				t.Fatalf("ls %v\n  got  %v\n  want %v", test.args, got, test.want)
			}
		})
	}
}

// -S gets its own fixture, with no subdirectory in it, because **a directory's
// apparent size is platform-dependent**: Windows reports 0 and Linux and macOS
// report the directory block. So a size-ordered listing that contains a
// directory puts it last on one platform and first on another, which is exactly
// how the first version of this test passed here and failed on both CI runners.
//
// The three files have distinct sizes *and* distinct times, so this still pins
// which key was used and the fact that the last option given wins.
func TestLs_sortsBySize(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	for name, file := range map[string]struct {
		content string
		age     time.Duration
	}{
		"a.txt": {content: "aaa", age: 0},
		"b.txt": {content: "b", age: 8 * 24 * time.Hour},
		"c.txt": {content: "cccccccc", age: 11 * 24 * time.Hour},
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(file.content), 0o644); err != nil {
			t.Fatal(err)
		}
		when := base.Add(file.age)
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		name string
		args []string
		want []string
	}{
		{name: "-S is largest first", args: []string{"-1", "-S"}, want: []string{"c.txt", "a.txt", "b.txt"}},
		{name: "-Sr is smallest first", args: []string{"-1", "-S", "-r"}, want: []string{"b.txt", "a.txt", "c.txt"}},
		{name: "size last wins over time", args: []string{"-1", "-t", "-S"}, want: []string{"c.txt", "a.txt", "b.txt"}},
		{name: "time last wins over size", args: []string{"-1", "-S", "-t"}, want: []string{"c.txt", "b.txt", "a.txt"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := lsNames(t, append(test.args, dir)...)
			if strings.Join(got, "|") != strings.Join(test.want, "|") {
				t.Fatalf("ls %v\n  got  %v\n  want %v", test.args, got, test.want)
			}
		})
	}
}

// A tie has to break somewhere, and it has to break the same way every run or
// two listings of an unchanged directory would differ.
func TestLs_sortTiesBreakByName(t *testing.T) {
	dir := t.TempDir()
	when := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	for _, name := range []string{"zebra", "alpha", "middle"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("same"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatal(err)
		}
	}
	for _, option := range []string{"-t", "-S"} {
		got := lsNames(t, "-1", option, dir)
		if want := "alpha|middle|zebra"; strings.Join(got, "|") != want {
			t.Fatalf("ls %s with every key equal gave %v, want %s", option, got, want)
		}
	}
}

func TestLs_almostAll(t *testing.T) {
	dir := lsSortFixture(t)

	// -A lists the hidden entry but not . and ..
	got := lsNames(t, "-1", "-A", dir)
	if want := ".hidden|a.txt|b.txt|c.txt|sub"; strings.Join(got, "|") != want {
		t.Fatalf("ls -A = %v, want %s", got, want)
	}

	// -a lists them, and beats -A in either order. That is busybox's rule
	// measured on 2026-08-22: `ls -A -a` and `ls -a -A` both show . and ..,
	// where GNU lets the later option win.
	for _, args := range [][]string{{"-1", "-a"}, {"-1", "-a", "-A"}, {"-1", "-A", "-a"}} {
		got := lsNames(t, append(args, dir)...)
		if len(got) < 2 || got[0] != "." || got[1] != ".." {
			t.Fatalf("ls %v = %v, want it to open with . and ..", args, got)
		}
	}
}

func TestLs_directoryItself(t *testing.T) {
	dir := lsSortFixture(t)

	// -d names the directory instead of listing it, which is what makes
	// `ls -d */` a list of directories rather than their contents.
	got := lsNames(t, "-d", dir)
	if strings.Join(got, "|") != dir {
		t.Fatalf("ls -d %s = %v, want the operand itself", dir, got)
	}

	// -d with the long form describes the directory, so the mode begins with d.
	stdout, _, err := runAppletWithInput(t, "", "ls", "-d", "-l", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stdout, "d") {
		t.Fatalf("ls -dl gave %q, want a line describing the directory itself", stdout)
	}
	if strings.Contains(stdout, "total ") {
		t.Fatalf("ls -dl printed a directory total: %q", stdout)
	}

	// A file operand is unaffected: -d is about not descending.
	got = lsNames(t, "-d", filepath.Join(dir, "a.txt"))
	if len(got) != 1 || !strings.HasSuffix(got[0], "a.txt") {
		t.Fatalf("ls -d on a file = %v", got)
	}
}

// -F appends one character saying what each entry is: `/` a directory, `@` a
// symlink, `*` an executable. On Windows the executable test is the extension,
// there being no mode bit to read.
func TestLs_classify(t *testing.T) {
	dir := lsSortFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "prog.exe"), []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := lsNames(t, "-1", "-F", dir)
	joined := strings.Join(got, "|")
	if !strings.Contains(joined, "sub/") {
		t.Fatalf("ls -F = %v, want sub marked as a directory", got)
	}
	if !strings.Contains(joined, "prog.exe*") {
		t.Fatalf("ls -F = %v, want prog.exe marked executable", got)
	}
	if !strings.Contains(joined, "a.txt|") {
		t.Fatalf("ls -F = %v, want a plain file left unmarked", got)
	}

	// -a marks the dot entries too, which is how the reference prints them.
	got = lsNames(t, "-1", "-a", "-F", dir)
	if got[0] != "./" || got[1] != "../" {
		t.Fatalf("ls -aF opened with %q %q, want ./ and ../", got[0], got[1])
	}
}

// -R descends, heading each directory with its path and a colon and separating
// them with a blank line. The header path is built from the operand as spelled,
// so `ls -R sub` says `sub:` and `sub/nested:`.
func TestLs_recursive(t *testing.T) {
	dir := lsSortFixture(t)
	stdout, stderr, err := runAppletWithInput(t, "", "ls", "-1", "-R", dir)
	if err != nil {
		t.Fatalf("ls -R: %v (%s)", err, stderr)
	}
	want := strings.Join([]string{
		dir + ":",
		"a.txt",
		"b.txt",
		"c.txt",
		"sub",
		"",
		dir + "/sub:",
		"n.txt",
		"nested",
		"",
		dir + "/sub/nested:",
		"",
	}, "\n")
	if stdout != want {
		t.Fatalf("ls -R gave\n%q\nwant\n%q", stdout, want)
	}
}

// A directory that cannot be read must not abandon the rest of the walk: the
// entries already found are still worth printing.
func TestLs_recursiveSkipsHiddenEntriesUnlessAsked(t *testing.T) {
	dir := lsSortFixture(t)
	stdout, _, err := runAppletWithInput(t, "", "ls", "-1", "-R", dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout, ".hidden") {
		t.Fatalf("ls -R listed a hidden entry without -a: %q", stdout)
	}
	stdout, _, err = runAppletWithInput(t, "", "ls", "-1", "-R", "-A", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, ".hidden") {
		t.Fatalf("ls -RA did not list the hidden entry: %q", stdout)
	}
	// -a would otherwise recurse into `.` and `..` for ever. The reference lists
	// them and does not follow them.
	stdout, _, err = runAppletWithInput(t, "", "ls", "-1", "-R", "-a", dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(stdout, "a.txt") > 1 {
		t.Fatalf("ls -Ra recursed into a dot entry: %q", stdout)
	}
}

// The options this build still does not have stay refused by name, so a script
// asking for one fails rather than quietly getting something else.
func TestLs_stillRefusesWhatItCannotDo(t *testing.T) {
	for _, option := range []string{"-i", "-n", "-u", "-c", "-X", "-v"} {
		if _, _, err := runAppletWithInput(t, "", "ls", option, "."); err == nil {
			t.Fatalf("ls %s was accepted, want a refusal", option)
		}
	}
}
