package runtime

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode cd refuses a path whose `..` follows a directory that is not there, as bash's
// does, which checks each such directory before it takes the `..` away: `cd nonexistent/..` is
// `No such file or directory`, status 1. busybox takes the `..` away first and goes to where
// the path ends, as nemosh and sh do. Measured 2026-11-22 with Git for Windows' bash 5.3 and
// busybox-w32; Oils' builtin-cd.test.sh, `cd BAD/..`.
func TestCD_bashModeChecksTheDirectoryBeforeDotDot(t *testing.T) {
	for _, test := range []struct {
		bash         bool
		code, stdout string
		said         string
	}{
		{bash: true, code: "cd nonexistent_ZZ/..; echo status=$?", stdout: "status=1\n", said: "cd: nonexistent_ZZ/..: No such file or directory\n"},
		{bash: true, code: "mkdir d; cd d/..; echo status=$?", stdout: "status=0\n"},
		{bash: true, code: "mkdir -p d/e; cd d/e/../../nope/..; echo status=$?", stdout: "status=1\n", said: "No such file or directory\n"},
		{bash: true, code: "cd -P nonexistent_ZZ/..; echo status=$?", stdout: "status=1\n", said: "No such file or directory\n"},
		{bash: false, code: "cd nonexistent_ZZ/..; echo status=$?", stdout: "status=0\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), "cd \""+filepath.ToSlash(t.TempDir())+"\"\n"+test.code+"\n")
		if stdout.String() != test.stdout || !strings.HasSuffix(stderr.String(), test.said) || test.said == "" && stderr.Len() != 0 {
			t.Errorf("bash mode %v, %s: stdout %q, stderr %q; want %q and %q", test.bash, test.code, stdout.String(), stderr.String(), test.stdout, test.said)
		}
	}
}
