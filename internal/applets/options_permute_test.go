package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

type permuteTestView struct {
	cwd string
	env map[string]string
}

func (v permuteTestView) WorkingDirectory() string { return v.cwd }
func (v permuteTestView) Environ() []string        { return nil }
func (v permuteTestView) LookupEnv(name string) (string, bool) {
	value, ok := v.env[name]
	return value, ok
}
func (v permuteTestView) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(v.cwd, path)
}

func runPermuted(t *testing.T, view permuteTestView, stdin string, args ...string) (string, string, error) {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup(args[0])
	if !ok {
		t.Fatalf("%s is not registered", args[0])
	}
	var stdout, stderr bytes.Buffer
	err := applet.Run(applets.WithProcessView(context.Background(), view), args[1:], strings.NewReader(stdin), &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

// An option may follow the operands, as busybox's getopt lets it: `wc f -l` counts lines and
// `grep b f -c` counts matches. Each option was taken for a file, so the command did the
// default thing and then failed on a file named -l.
func TestAppletOptions_mayFollowTheOperands(t *testing.T) {
	dir := t.TempDir()
	view := permuteTestView{cwd: dir}
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("b\na\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args  []string
		stdin string
		want  string
	}{
		{[]string{"wc", "f", "-l"}, "", "3 f\n"},
		{[]string{"cat", "f", "-n"}, "", "     1\tb\n     2\ta\n     3\tb\n"},
		{[]string{"grep", "b", "f", "-c"}, "", "2\n"},
		{[]string{"sed", "2p", "f", "-n"}, "", "a\n"},
		{[]string{"tail", "f", "-n1"}, "", "b\n"},
		{[]string{"sort", "f", "-u"}, "", "a\nb\n"},
		{[]string{"ls", "f", "-1"}, "", "f\n"},
		{[]string{"cat", "--", "f"}, "", "b\na\nb\n"},
		// busybox reads these in order, with a + in their getopt strings: the words after
		// the first operand belong to the command xargs runs.
		{[]string{"xargs", "echo", "-n"}, "x\n", "x"},
	} {
		stdout, stderr, err := runPermuted(t, view, test.stdin, test.args...)
		if stdout != test.want || stderr != "" || err != nil {
			t.Errorf("%q: got %q, %q, %v; want %q", test.args, stdout, stderr, err, test.want)
		}
	}

	if _, _, err := runPermuted(t, view, "", "touch", "t", "-c"); err != nil {
		t.Fatalf("touch t -c: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "t")); !os.IsNotExist(err) {
		t.Errorf("touch t -c made t (%v); -c says not to", err)
	}
	if _, _, err := runPermuted(t, view, "", "mkdir", "d/x", "-p"); err != nil {
		t.Fatalf("mkdir d/x -p: %v", err)
	}
	if info, err := os.Stat(filepath.Join(dir, "d", "x")); err != nil || !info.IsDir() {
		t.Errorf("mkdir d/x -p made no d/x: %v", err)
	}
	if _, stderr, err := runPermuted(t, view, "", "rm", "gone", "-f"); err != nil || stderr != "" {
		t.Errorf("rm gone -f: %q, %v; want -f to keep it quiet", stderr, err)
	}
}

// head reads its own options, and they end at its first file, as busybox's do. Under
// POSIXLY_CORRECT every applet's do, as getopt's.
func TestAppletOptions_endAtTheFirstOperandWhereBusyboxsDo(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("b\na\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	strict := permuteTestView{cwd: dir, env: map[string]string{"POSIXLY_CORRECT": "1"}}
	for _, test := range []struct {
		view       permuteTestView
		args       []string
		wantStdout string
		wantErr    string
	}{
		{permuteTestView{cwd: dir}, []string{"head", "f", "-n1"}, "==> f <==\nb\na\n", "head: -n1: No such file or directory"},
		{strict, []string{"cat", "f", "-n"}, "b\na\n", "cannot open '-n': No such file or directory"},
		{strict, []string{"grep", "b", "f", "-c"}, "f:b\n", "-c: No such file or directory"},
	} {
		stdout, stderr, err := runPermuted(t, test.view, "", test.args...)
		got := strings.TrimSpace(stderr)
		if got == "" && err != nil {
			got = err.Error()
		}
		if stdout != test.wantStdout || !strings.Contains(got, test.wantErr) {
			t.Errorf("%q: got %q, %q; want %q and %q", test.args, stdout, got, test.wantStdout, test.wantErr)
		}
	}
}

// A long option an applet does not have is named whole, as GNU's getopt_long names it and cp
// and mv here already did. It was read as a cluster of letters and refused as the first, `-`.
func TestAppletOptions_nameAnUnknownLongOptionWhole(t *testing.T) {
	view := permuteTestView{cwd: t.TempDir()}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"du", "--apparent-size"}, "unrecognized option '--apparent-size'"},
		{[]string{"wc", "x", "--lines"}, "unrecognized option '--lines'"},
		{[]string{"xargs", "--no-run-if-empty", "echo"}, "unrecognized option '--no-run-if-empty'"},
		// sort says so itself, and ends 2, as for any other bad option.
		{[]string{"sort", "--foo=bar"}, "sort: unrecognized option '--foo=bar'\n"},
	} {
		_, stderr, err := runPermuted(t, view, "", test.args...)
		if reported := stderr; err == nil || reported != test.want && err.Error() != test.want {
			t.Errorf("%q: %q, %v; want %s", test.args, stderr, err, test.want)
		}
	}
}
