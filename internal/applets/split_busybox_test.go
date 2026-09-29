package applets_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// split writes the bytes as they come, as busybox's does, each case measured against
// busybox-w32: a CRLF line stays one, the last line keeps what it ended with, -b counts bytes
// with its suffixes, -a sets how many letters, and of -l and -b the last given has its number
// counted as -b says. It joined each file's lines with a newline, so CRLF came out LF and an
// unterminated last line terminated, and took -l alone.
func TestSplit_writesTheBytesAsBusyboxDoes(t *testing.T) {
	const input = "l1\r\nl2\r\nl3\nl4\nl5"
	for _, test := range []struct {
		args []string
		want map[string]string
	}{
		{[]string{"-l", "2", "input"}, map[string]string{"xaa": "l1\r\nl2\r\n", "xab": "l3\nl4\n", "xac": "l5"}},
		{[]string{"-l", "1", "input", "part"}, map[string]string{"partaa": "l1\r\n", "partab": "l2\r\n", "partac": "l3\n", "partad": "l4\n", "partae": "l5"}},
		{[]string{"input"}, map[string]string{"xaa": input}},
		{[]string{"-b", "4", "input"}, map[string]string{"xaa": "l1\r\n", "xab": "l2\r\n", "xac": "l3\nl", "xad": "4\nl5"}},
		{[]string{"-a", "1", "-l", "3", "input"}, map[string]string{"xa": "l1\r\nl2\r\nl3\n", "xb": "l4\nl5"}},
		{[]string{"-l", "5", "-b", "7", "input"}, map[string]string{"xaa": "l1\r\nl2\r", "xab": "\nl3\nl4\n", "xac": "l5"}},
		{[]string{"-b", "7", "-l", "8", "input"}, map[string]string{"xaa": "l1\r\nl2\r\n", "xab": "l3\nl4\nl5"}},
		{[]string{"-b", "1k", "big"}, map[string]string{"xaa": strings.Repeat("z", 1024), "xab": strings.Repeat("z", 1024), "xac": strings.Repeat("z", 952)}},
	} {
		dir := t.TempDir()
		view := permuteTestView{cwd: dir}
		if err := os.WriteFile(filepath.Join(dir, "input"), []byte(input), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "big"), []byte(strings.Repeat("z", 3000)), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, stderr, err := runPermuted(t, view, "", append([]string{"split"}, test.args...)...); err != nil || stderr != "" {
			t.Fatalf("split %q: %q, %v", test.args, stderr, err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, entry := range entries {
			if name := entry.Name(); name != "input" && name != "big" {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		if len(names) != len(test.want) {
			t.Errorf("split %q wrote %q, want %d files", test.args, names, len(test.want))
			continue
		}
		for _, name := range names {
			body, _ := os.ReadFile(filepath.Join(dir, name))
			if want, ok := test.want[name]; !ok || string(body) != want {
				t.Errorf("split %q: %s holds %q, want %q", test.args, name, body, want)
			}
		}
	}
}

// Its numbers are busybox's, and a count of 0 is refused in GNU's words: busybox's -b 0 makes
// every name it has, empty, and its -l 0 puts everything in the first. Running out of names is
// busybox's `suffixes exhausted`, with the files before it written.
func TestSplit_refusesAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	if err := os.WriteFile(filepath.Join(dir, "input"), []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-l", "x", "input"}, "invalid number 'x'"},
		{[]string{"-b", "2x", "input"}, "invalid number '2x'"},
		{[]string{"-a", "x", "input"}, "invalid number 'x'"},
		{[]string{"-l", "+2", "input"}, "invalid number '+2'"},
		{[]string{"-l", "0", "input"}, "invalid number of lines: '0'"},
		{[]string{"-b", "0", "input"}, "invalid number of bytes: '0'"},
		{[]string{"-b", "18446744073709551615k", "input"}, "number 18446744073709551615k is not in 0..18446744073709551615 range"},
		{[]string{"-l", "9223372036854775808", "input"}, "number 9223372036854775808 is not in 0..9223372036854775807 range"},
		{[]string{"-l", "2", "missing"}, "cannot open 'missing': No such file or directory"},
		{[]string{"-l", "2", "input", "p", "extra"}, "extra operand 'extra'"},
		{[]string{"-a", "1", "-l", "1", "input", "q"}, ""},
		{[]string{"-a", "300", "input"}, "suffix too long"},
	} {
		_, _, err := runPermuted(t, view, "", append([]string{"split"}, test.args...)...)
		if test.want == "" {
			if err != nil {
				t.Errorf("split %q: %v", test.args, err)
			}
			continue
		}
		if err == nil || err.Error() != test.want {
			t.Errorf("split %q: %v, want %q", test.args, err, test.want)
		}
	}
	// Three letters' worth of names with one letter each: qa to qz is 26, and a 27th is refused.
	lines := strings.Repeat("x\n", 27)
	if err := os.WriteFile(filepath.Join(dir, "many"), []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runPermuted(t, view, "", "split", "-a", "1", "-l", "1", "many", "r"); err == nil || err.Error() != "suffixes exhausted" {
		t.Errorf("split -a 1 -l 1 of 27 lines: %v, want suffixes exhausted", err)
	}
	if body, err := os.ReadFile(filepath.Join(dir, "rz")); err != nil || string(body) != "x\n" {
		t.Errorf("split -a 1 wrote rz as %q, %v; want the 26th line", body, err)
	}
}
