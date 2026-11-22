package runtime

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode PPID is an integer and read-only, `declare -ir PPID="..."`, and `$_` starts as the
// path that ran the shell, so `set -u` finds it set. busybox's PPID takes an assignment and its
// `$_` is unset until a command has run, as nemosh's and sh's are. Measured 2026-11-22 with Git
// for Windows' bash 5.3; Oils' vars-special.test.sh.
func TestBashStartupVars_ppidReadOnlyAndUnderscoreSet(t *testing.T) {
	// An `_` the environment hands the test, as a bash that ran it does, is a shell's `$_` from
	// the start in both modes; this asks about a start without one.
	t.Setenv("_", "")
	if err := os.Unsetenv("_"); err != nil {
		t.Fatal(err)
	}
	const code = "echo \"under=${_:+set}\"\nPPID=xx\necho \"st=$?\"\ndeclare -p PPID | cut -d= -f1"
	for _, test := range []struct {
		bash         bool
		stdout, said string
	}{
		{true, "under=set\nst=1\ndeclare -ir PPID\n", "PPID: readonly variable"},
		{false, "under=\nst=0\ndeclare -- PPID\n", ""},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), code+"\n")
		if stdout.String() != test.stdout || !strings.Contains(stderr.String(), test.said) {
			t.Errorf("bash mode %v: stdout %q, stderr %q; want %q and %q", test.bash, stdout.String(), stderr.String(), test.stdout, test.said)
		}
	}
}
