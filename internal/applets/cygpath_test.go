package applets

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// cygpath is Git for Windows' -- MSYS2's -- over this shell's path model: -u the Unix spelling,
// /c/Users; -w the Windows one with backslashes; -m with forward slashes; -a absolute; -p a list,
// a Unix one split at : for -w and -m and a Windows one at ; for -u. A relative path stays
// relative unless -a. Each answer is Git's cygpath 3.6.10's, measured 2026-11-21. zoxide's bash
// init calls `cygpath -w "$(pwd -L)"` before every prompt, and found none here.
func TestCygpath_convertsAsGitsDoes(t *testing.T) {
	dir := t.TempDir()
	ctx := WithProcessView(context.Background(), hostProcessView{cwd: dir})
	run := func(args ...string) (string, string, int) {
		applet, found := DefaultRegistry.Lookup("cygpath")
		if !found {
			t.Fatal("cygpath is not registered")
		}
		var stdout, stderr strings.Builder
		err := applet.Run(ctx, args, strings.NewReader(""), &stdout, &stderr)
		status := 0
		if err != nil {
			status = 1
			if code, carried := StatusCode(err); carried {
				status = code
			}
		}
		return stdout.String(), stderr.String(), status
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"-u", `C:\Users\nemo`}, want: "/c/Users/nemo\n"},
		{args: []string{"-w", "/c/Users/nemo"}, want: `C:\Users\nemo` + "\n"},
		{args: []string{"-m", "/c/Users/nemo"}, want: "C:/Users/nemo\n"},
		{args: []string{"C:/Users/nemo"}, want: "/c/Users/nemo\n"},
		{args: []string{"-w", "foo/bar"}, want: `foo\bar` + "\n"},
		{args: []string{"-u", `foo\bar`}, want: "foo/bar\n"},
		{args: []string{"-aw", "foo/bar"}, want: filepath.Join(dir, "foo", "bar") + "\n"},
		{args: []string{"-pw", "/c/a:/c/b"}, want: `C:\a;C:\b` + "\n"},
		{args: []string{"-pu", `C:\a;C:\b`}, want: "/c/a:/c/b\n"},
		{args: []string{"-u", `\\server\share\x`}, want: "//server/share/x\n"},
		{args: []string{"-w", "//server/share/x"}, want: `\\server\share\x` + "\n"},
		{args: []string{"--windows", "/c/a", "/c/b"}, want: `C:\a` + "\n" + `C:\b` + "\n"},
	} {
		if got, stderr, status := run(test.args...); got != test.want || status != 0 {
			t.Errorf("cygpath %q: %q, status %d, stderr %q; want %q", test.args, got, status, stderr, test.want)
		}
	}
	if _, _, status := run(); status == 0 {
		t.Errorf("cygpath with no path: status %d; want a usage error", status)
	}
	if _, _, status := run("-l", "/c/a"); status == 0 {
		t.Error("cygpath -l, which this one has not got, was taken")
	}
}
