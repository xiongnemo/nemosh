package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode eval takes no option, so `eval -z` is bash's `eval: -z: invalid option` and its
// usage, status 2; `eval -` runs a command named -, as there, and `eval --` ends the options
// already. busybox runs -z as a command, 127, and so do nemosh and sh. Measured 2026-11-22 with
// Git for Windows' bash 5.3; Oils' builtin-eval-source.test.sh.
func TestEvalOption_bashModeRefusesOne(t *testing.T) {
	const code = "eval -z\necho st=$?\neval -- echo ok"
	for _, test := range []struct {
		bash           bool
		stdout, stderr string
	}{
		{true, "st=2\nok\n", "nemosh: line 1: eval: -z: invalid option\neval: usage: eval [arg ...]\n"},
		{false, "st=127\n", ""},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), code+"\n")
		if stdout.String() != test.stdout || test.bash && stderr.String() != test.stderr {
			t.Errorf("bash mode %v: stdout %q, stderr %q; want %q and %q", test.bash, stdout.String(), stderr.String(), test.stdout, test.stderr)
		}
	}
}
