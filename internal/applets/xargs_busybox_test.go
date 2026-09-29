package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// xargs is busybox's, each case measured against busybox-w32: words quoted and escaped as its
// process_stdin reads them, -0, -I and -i reading lines, -n and -s filling each command line as
// busybox's room is counted, -E and -e ending the input, -a, -r and --no-run-if-empty, and its
// statuses. It split at blanks alone and took -0 -n -I -r -t and no others.
func TestXargs_buildsCommandLinesAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "argsfile"), []byte("one two\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	view := permuteTestView{cwd: dir}
	for _, test := range []struct {
		args        []string
		input, want string
	}{
		{[]string{"echo"}, "a b c\n", "a b c\n"},
		{nil, "a b c\n", "a b c\n"},
		{[]string{"echo"}, "\"a b\" c\n", "a b c\n"},
		{[]string{"echo"}, "'a b' c\n", "a b c\n"},
		{[]string{"echo"}, "a\\ b c\n", "a b c\n"},
		{[]string{"echo"}, "a\"b c\"d e\n", "ab cd e\n"},
		{[]string{"echo"}, "\\\\ x\n", "\\ x\n"},
		{[]string{"echo"}, "\"\" x\n", "x\n"},
		{[]string{"-n", "2", "echo"}, "a b c d e\n", "a b\nc d\ne\n"},
		{[]string{"-n2", "echo"}, "a b c\n", "a b\nc\n"},
		{[]string{"-s", "10", "echo"}, "aa bb cc dd ee\n", "aa\nbb\ncc\ndd\nee\n"},
		{[]string{"-s", "12", "echo"}, "\"aa bb\" \"cc dd\" ee\n", "aa bb\ncc dd\nee\n"},
		{[]string{"-s", "13", "-n", "2", "echo"}, "a b c d e f\n", "a b\nc d\ne f\n"},
		{[]string{"-E", "stop", "echo"}, "a b stop c d\n", "a b\n"},
		{[]string{"-n", "2", "-E", "c", "echo"}, "a b c d\n", "a b\n"},
		{[]string{"-E", "", "echo"}, "a _ b\n", "a _ b\n"},
		{[]string{"-estop", "echo"}, "a stop b\n", "a\n"},
		{[]string{"-e", "echo"}, "a stop b\n", "a stop b\n"},
		{[]string{"-I", "{}", "echo", "[{}]"}, " x y \n  z\n\nw", "[x y ]\n[z]\n[w]\n"},
		{[]string{"-I%", "echo", "%-%"}, "a b\nc\n", "a b-a b\nc-c\n"},
		{[]string{"-i", "echo", "[{}]"}, "p q\nr\n", "[p q]\n[r]\n"},
		{[]string{"-ix", "echo", "[x]"}, "p\n", "[p]\n"},
		{[]string{"-I", "{}", "{}"}, "echo\n", "\n"},
		{[]string{"-0", "echo"}, "a b\x00c\x00", "a b c\n"},
		{[]string{"-0", "-n", "1", "echo"}, "a\x00\x00b", "a\n\nb\n"},
		{[]string{"-0", "-I", "{}", "echo", "[{}]"}, "x y\x00z\x00", "[x y]\n[z]\n"},
		{[]string{"-r", "echo", "nothing"}, "", ""},
		{[]string{"--no-run-if-empty", "echo", "nothing"}, "", ""},
		{[]string{"echo", "empty"}, "", "empty\n"},
		{[]string{"-a", "argsfile", "echo"}, "ignored\n", "one two three\n"},
		{[]string{"-x", "echo"}, "a\n", "a\n"},
		{[]string{"-n", "1", "--", "echo"}, "x y\n", "x\ny\n"},
		{[]string{"echo", "-n"}, "a b\n", "a b"},
	} {
		stdout, stderr, err := runPermuted(t, view, test.input, append([]string{"xargs"}, test.args...)...)
		if err != nil || stderr != "" || stdout != test.want {
			t.Errorf("xargs %q < %q: %q, %q, %v; want %q", test.args, test.input, stdout, stderr, err, test.want)
		}
	}
}

// The refusals and the statuses are busybox's: 123 for a command that failed, 127 for one
// there is no applet for, and the size limits' two errors.
func TestXargs_failsAsBusyboxDoes(t *testing.T) {
	view := permuteTestView{cwd: t.TempDir()}
	for _, test := range []struct {
		args          []string
		input, want   string
		status        int
		stdout, noted string
	}{
		{args: []string{"echo"}, input: "\"a b\n", want: "unmatched double quote", status: 1},
		{args: []string{"echo"}, input: "'a b\n", want: "unmatched single quote", status: 1},
		{args: []string{"-n", "0", "echo"}, input: "a\n", want: "number 0 is not in 1..2147483647 range", status: 1},
		{args: []string{"-n", "x", "echo"}, input: "a\n", want: "invalid number 'x'", status: 1},
		{args: []string{"-n1r", "echo"}, input: "a\n", want: "invalid number '1r'", status: 1},
		{args: []string{"-s", "8", "echo"}, input: "aaaaaaaaaa\n", want: "argument line too long", status: 1},
		{args: []string{"-s", "5", "echo"}, input: "a\n", want: "can't fit single argument within argument list size limit", status: 1},
		{args: []string{"-s", "12", "-I", "{}", "echo", "{}"}, input: "abcdef\nabcdefg\n", want: "argument line too long", status: 1},
		{args: []string{"-s", "13", "-I", "{}", "echo", "{}"}, input: "abcd\nabcdefg\n", want: "argument line too long", status: 1, stdout: "abcd\n"},
		{args: []string{"-P", "x", "echo"}, input: "a\n", want: "invalid number 'x'", status: 1},
		{args: []string{"-a", "missing", "echo"}, want: "cannot open 'missing': No such file or directory", status: 1},
		{args: []string{"nosuchcmd"}, input: "a\n", want: "nosuchcmd: No such file or directory", status: 127},
		{args: []string{"-n", "1", "false"}, input: "a\nb\n", status: 123},
		{args: []string{"-I", "", "echo", "x"}, input: "ab\n", want: "replacement string cannot be empty", status: 1},
	} {
		stdout, _, err := runPermuted(t, view, test.input, append([]string{"xargs"}, test.args...)...)
		status, _ := applets.StatusCode(err)
		if err != nil && status == 0 {
			status = 1
		}
		message, _ := applets.StatusMessage(err)
		if err != nil && message == "" && status == 1 {
			message = err.Error()
		}
		if status != test.status || message != test.want || stdout != test.stdout {
			t.Errorf("xargs %q < %q: %q, %v (status %d); want %q, status %d", test.args, test.input, stdout, err, status, test.want, test.status)
		}
	}
}

// -P runs the command lines side by side, each Write whole, and still ends 123 for one that
// failed. It was refused.
func TestXargs_runsSideBySideWithP(t *testing.T) {
	applet, _ := applets.DefaultRegistry.Lookup("xargs")
	var stdout, stderr bytes.Buffer
	input := strings.Repeat("word\n", 40)
	if err := applet.Run(context.Background(), []string{"-P", "8", "-n", "1", "echo"}, strings.NewReader(input), &stdout, &stderr); err != nil {
		t.Fatalf("xargs -P 8: %v (%s)", err, stderr.String())
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	sort.Strings(lines)
	if len(lines) != 40 || lines[0] != "word" || lines[39] != "word" {
		t.Fatalf("xargs -P 8 printed %q, want forty lines of word", stdout.String())
	}
	err := applet.Run(context.Background(), []string{"-P", "0", "-n", "1", "false"}, strings.NewReader("a\nb\nc\n"), &stdout, &stderr)
	if status, _ := applets.StatusCode(err); status != 123 {
		t.Fatalf("xargs -P 0 false: %v, want status 123", err)
	}
}
