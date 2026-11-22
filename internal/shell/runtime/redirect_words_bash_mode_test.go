package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode a redirection that cannot be made is said in bash's words, the file and why,
// `/no/such/dir/x: No such file or directory`, `f: cannot overwrite existing file` under set -C,
// and `9: Bad file descriptor`. busybox's, which nemosh and sh keep, are `cannot create
// /no/such/dir/x: nonexistent directory`, `cannot create f: File exists` and `dup2(9,1): Bad file
// descriptor`. Measured 2026-11-22 with Git for Windows' bash 5.3.
func TestRedirectWords_bashModeAreBashs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		code, said string
		bash       bool
	}{
		{"echo hi > /no/such/dir/x", "nemosh: line 2: /no/such/dir/x: No such file or directory\n", true},
		{"cat < /no/such/file", "nemosh: line 2: /no/such/file: No such file or directory\n", true},
		{"set -C; : > f", "nemosh: line 2: f: cannot overwrite existing file\n", true},
		{"echo hi >&9", "nemosh: line 2: 9: Bad file descriptor\n", true},
		{"echo hi > /no/such/dir/x", "nemosh: line 2: cannot create /no/such/dir/x: nonexistent directory\n", false},
		{"echo hi >&9", "nemosh: line 2: dup2(9,1): Bad file descriptor\n", false},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), "cd '"+filepath.ToSlash(dir)+"'\n"+test.code+"\n")
		if stderr.String() != test.said {
			t.Errorf("bash mode %v, %q: stderr %q, want %q", test.bash, test.code, stderr.String(), test.said)
		}
	}
}
