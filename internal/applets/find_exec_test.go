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

// runFindWith runs find in dir with stdin, as -ok reads its answers from it.
func runFindWith(t *testing.T, dir, stdin string, args ...string) (string, string, error) {
	t.Helper()
	applet, _ := applets.DefaultRegistry.Lookup("find")
	var stdout, stderr bytes.Buffer
	ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: dir})
	err := applet.Run(ctx, args, strings.NewReader(stdin), &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func findExecFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"f1.txt", "d/f2.log", "d/e/f3.txt"} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// -exec and -ok run an applet for each entry, every `{}` in a word its path, true when it ends
// 0; `-exec ... {} +` runs it once for the entries it gathered, and its failure is find's status
// 1; a name no applet has is said and is false. They were refused by name. Each answer is
// busybox-w32's, measured.
func TestFind_execRunsAnAppletForTheEntries(t *testing.T) {
	dir := findExecFixture(t)
	for _, test := range []struct {
		stdin, stdout, stderr string
		args                  []string
		fails                 bool
	}{
		{args: []string{".", "-name", "*.txt", "-exec", "echo", "X", "{}", "Y", ";"}, stdout: "X ./d/e/f3.txt Y\nX ./f1.txt Y\n"},
		{args: []string{".", "-name", "*.txt", "-exec", "echo", "[{}]", ";"}, stdout: "[./d/e/f3.txt]\n[./f1.txt]\n"},
		{args: []string{".", "-type", "f", "-exec", "echo", "A", "{}", "+"}, stdout: "A ./d/e/f3.txt ./d/f2.log ./f1.txt\n"},
		{args: []string{".", "-type", "f", "-exec", "false", ";"}},
		{args: []string{".", "-type", "f", "-exec", "false", "{}", "+"}, fails: true},
		{args: []string{".", "-name", "f1*", "-exec", "nosuchcmd_zz", "{}", ";"}, stderr: "find: nosuchcmd_zz: No such file or directory\n"},
		{args: []string{".", "-name", "f1.txt", "-exec", "false", ";", "-o", "-name", "f1.txt", "-print"}, stdout: "./f1.txt\n"},
		{stdin: "n\n", args: []string{".", "-name", "f1.txt", "-ok", "echo", "OK", "{}", ";"}, stderr: "echo OK ./f1.txt ?"},
		{stdin: "y\n", args: []string{".", "-name", "f1.txt", "-ok", "echo", "OK", "{}", ";"}, stdout: "OK ./f1.txt\n", stderr: "echo OK ./f1.txt ?"},
	} {
		stdout, stderr, err := runFindWith(t, dir, test.stdin, test.args...)
		if stdout != test.stdout || stderr != test.stderr || (err != nil) != test.fails {
			t.Errorf("find %q = %q, %q, %v; want %q, %q, failing %v", test.args, stdout, stderr, err, test.stdout, test.stderr, test.fails)
		}
	}
}

// -delete removes each entry, a directory after what is in it and only when it is empty, says
// a failure and goes on, status 0, as busybox's does. It was refused by name.
func TestFind_deleteRemovesTheEntries(t *testing.T) {
	dir := findExecFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "d", "keep.md"), []byte("k"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runFindWith(t, dir, "", "d", "-name", "*.txt", "-delete", "-print")
	if stdout != "d/e/f3.txt\n" || stderr != "" || err != nil {
		t.Errorf("-name -delete -print: %q, %q, %v", stdout, stderr, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "d", "e", "f3.txt")); !os.IsNotExist(err) {
		t.Errorf("d/e/f3.txt is still there: %v", err)
	}
	_, stderr, err = runFindWith(t, dir, "", "d", "-name", "d", "-delete")
	if !strings.HasPrefix(stderr, "find: d: ") || err != nil {
		t.Errorf("a directory with files in it: %q, %v; want it said and status 0", stderr, err)
	}
	if _, _, err := runFindWith(t, dir, "", "d", "-delete"); err != nil {
		t.Errorf("find d -delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "d")); !os.IsNotExist(err) {
		t.Errorf("d is still there after find d -delete: %v", err)
	}
}
