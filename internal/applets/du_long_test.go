package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// du takes GNU's long options, which every Debian administrator types: `du -h --max-depth=1`,
// `du --exclude=node_modules`, `du --apparent-size`. It took none, so each was `unknown option`.
func duTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, size := range map[string]int{"a.txt": 100, "b.log": 300, "sub/c.txt": 2000, "node_modules/d.js": 5000} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.Repeat("x", size)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runDu(t *testing.T, args ...string) string {
	t.Helper()
	stdout, stderr, err := runAppletWithInput(t, "", "du", args...)
	if err != nil {
		t.Fatalf("du %q: %v (%s)", args, err, stderr)
	}
	return stdout
}

func TestDu_longOptionsAreTheLetters(t *testing.T) {
	root := duTree(t)
	for _, pair := range [][2][]string{
		{{"--max-depth=0", root}, {"-d", "0", root}},
		{{"--max-depth", "1", root}, {"-d1", root}},
		{{"--summarize", root}, {"-s", root}},
		{{"--all", "-b", root}, {"-a", "-b", root}},
		{{"--bytes", "-s", root}, {"-b", "-s", root}},
		{{"--human-readable", "-s", root}, {"-h", "-s", root}},
		{{"--total", "-sb", root, root + "/sub"}, {"-c", "-sb", root, root + "/sub"}},
		{{"--one-file-system", "--count-links", "--dereference", "--dereference-args", "-s", root}, {"-x", "-l", "-L", "-H", "-s", root}},
		{{"--summ", root}, {"-s", root}},
	} {
		if got, want := runDu(t, pair[0]...), runDu(t, pair[1]...); got != want {
			t.Errorf("du %q:\n%s\nwant what du %q says:\n%s", pair[0], got, pair[1], want)
		}
	}
}

// --exclude leaves out what matches, by name or, with a slash in it, by the path du prints, and
// counts none of it; -X reads the patterns a line at a time from a file.
func TestDu_excludes(t *testing.T) {
	root := duTree(t)
	patterns := filepath.Join(t.TempDir(), "patterns")
	if err := os.WriteFile(patterns, []byte("*.log\nnode_modules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	slash := filepath.ToSlash(root)
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"-sb", "--exclude=*.log", root}, want: "7100\t" + root + "\n"},
		{args: []string{"-sb", "--exclude", "node_modules", root}, want: "2400\t" + root + "\n"},
		{args: []string{"-sb", "--exclude=*.log", "--exclude=node_modules", root}, want: "2100\t" + root + "\n"},
		{args: []string{"-sb", "-X", patterns, root}, want: "2100\t" + root + "\n"},
		{args: []string{"-sb", "--exclude-from=" + patterns, root}, want: "2100\t" + root + "\n"},
		{args: []string{"-sb", "--exclude=" + slash + "/sub/*", slash}, want: "5400\t" + slash + "\n"},
	} {
		if got := runDu(t, test.args...); got != test.want {
			t.Errorf("du %q = %q, want %q", test.args, got, test.want)
		}
	}
}

// --apparent-size counts what the files hold rather than what they take, and rounds the sum up
// to the unit as GNU's does: 7,400 bytes are 8K, where the blocks they take may be more.
func TestDu_apparentSize(t *testing.T) {
	root := duTree(t)
	if got, want := runDu(t, "-s", "--apparent-size", root), "8\t"+root+"\n"; got != want {
		t.Errorf("du -s --apparent-size = %q, want %q", got, want)
	}
	if got, want := runDu(t, "-s", "--apparent-size", root+"/a.txt"), "1\t"+root+"/a.txt\n"; got != want {
		t.Errorf("du -s --apparent-size of 100 bytes = %q, want %q", got, want)
	}
	if got, want := runDu(t, "-sb", "--apparent-size", root), "7400\t"+root+"\n"; got != want {
		t.Errorf("du -sb --apparent-size = %q, want %q", got, want)
	}
}

func TestDu_refusesWhatItDoesNotHave(t *testing.T) {
	for _, test := range []struct {
		args []string
		says string
	}{
		{args: []string{"--bogus"}, says: "unknown option -- bogus"},
		{args: []string{"--max-depth=x", "."}, says: "invalid number 'x'"},
		{args: []string{"-X", "zzznosuchfile", "."}, says: "zzznosuchfile"},
	} {
		_, stderr, err := runAppletWithInput(t, "", "du", test.args...)
		if reported := stderr + errorText(err); !strings.Contains(reported, test.says) {
			t.Errorf("du %q reported %q, want %q", test.args, reported, test.says)
		}
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
