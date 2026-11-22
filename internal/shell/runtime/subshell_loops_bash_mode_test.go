package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode a subshell, `( continue )`, and a pipeline's stage see none of the shell's loops,
// as a function does not: break and continue there are bash's note that they mean nothing, and
// the subshell goes on. A command substitution sees them, and its break ends it, in bash as in
// busybox, where a subshell and a stage see them too, as nemosh and sh do. Measured 2026-11-22
// with Git for Windows' bash 5.3; Oils' loop.test.sh.
func TestSubshellLoops_bashModeSeesNone(t *testing.T) {
	const code = `for i in 1 2; do ( continue; echo "sub $i" ); echo "after $i"; done
for i in 1; do echo p | { continue; echo pipe; }; echo "after pipe"; done
for i in 1; do x=$(break; echo inside); echo "[$x]"; done`
	for _, test := range []struct {
		bash   bool
		stdout string
	}{
		{true, "sub 1\nafter 1\nsub 2\nafter 2\npipe\nafter pipe\n[]\n"},
		{false, "after 1\nafter 2\nafter pipe\n[]\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), code+"\n")
		if stdout.String() != test.stdout {
			t.Errorf("bash mode %v: stdout %q, stderr %q; want %q", test.bash, stdout.String(), stderr.String(), test.stdout)
		}
	}
}
