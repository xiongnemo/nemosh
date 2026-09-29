package runtime_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// A redirection that cannot be made is worded as busybox's openredirect words it: `cannot create`
// for one that writes and `cannot open` for `<`, with errmsg's `nonexistent directory` and `no
// such file` for what is not there, and `dup2(5,1): Bad file descriptor` for a descriptor that is
// not open. It named the descriptor, the host path and Windows' own sentence, and a directory
// read through `<` failed at the first read with `Incorrect function`.
func TestRuntime_aFailedRedirectionIsWordedAsBusyboxWordsIt(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	script := "cd '" + filepath.ToSlash(dir) + `'
echo x > missing/f
cat < missing
echo x > .
echo x >> .
cat < .
echo x 3<> missing/f
set -C; echo a > f; echo b > f; set +C
echo x >&5
cat <&7
x=$(< missing)
echo "status=$?"
`
	rt.RunScript(context.Background(), script)
	want := "nemosh: cannot create missing/f: nonexistent directory\n" +
		"nemosh: cannot open missing: no such file\n" +
		"nemosh: cannot create .: Is a directory\n" +
		"nemosh: cannot create .: Is a directory\n" +
		"nemosh: cannot open .: Is a directory\n" +
		"nemosh: cannot create missing/f: nonexistent directory\n" +
		"nemosh: cannot create f: File exists\n" +
		"nemosh: dup2(5,1): Bad file descriptor\n" +
		"nemosh: dup2(7,0): Bad file descriptor\n" +
		"nemosh: cannot open missing: no such file\n"
	if got := stderr.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if got := stdout.String(); got != "status=1\n" {
		t.Errorf("stdout %q, want status=1", got)
	}
}
