package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// `. FILE` that cannot be read is a shell error, as busybox's dotcmd and POSIX 2.8.1 have it: the
// script ends there with 2, a subshell ends, and through `command` it is a plain status 2 and the
// script goes on. A directory is refused as one. It went on with status 1, and named the host
// path the file had been resolved to.
func TestRuntime_aDotFileThatCannotBeReadEndsTheScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	script := "cd '" + filepath.ToSlash(dir) + `'
command . ./missing; echo "command=$?"
( . ./missing ); echo "subshell=$?"
( source ./sub ); echo "directory=$?"
. ./missing
echo "not reached"
`
	status := rt.RunScript(context.Background(), script)
	if status != 2 {
		t.Errorf("status %d, want 2", status)
	}
	if got, want := stdout.String(), "command=2\nsubshell=2\ndirectory=2\n"; got != want {
		t.Errorf("stdout %q, want %q", got, want)
	}
	want := "nemosh: line 2: .: cannot open ./missing: no such file\n" +
		"nemosh: line 3: .: cannot open ./missing: no such file\n" +
		"nemosh: line 4: source: cannot open ./sub: Is a directory\n" +
		"nemosh: line 5: .: cannot open ./missing: no such file\n"
	if got := stderr.String(); got != want {
		t.Errorf("stderr\n%s\nwant\n%s", got, want)
	}
}
