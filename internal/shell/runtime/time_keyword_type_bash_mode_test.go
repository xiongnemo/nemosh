package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode `time` is a keyword to type and command, as it is to bash: `type -t time` says
// keyword, `type time` and `command -V time` "time is a shell keyword". busybox has no such
// keyword and calls its applet "a builtin applet", which nemosh and sh keep. Measured 2026-11-22
// with Git for Windows' bash 5.3; Oils' builtin-type-bash.test.sh.
func TestTimeKeywordType_bashModeIsAKeyword(t *testing.T) {
	const code = "type -t time; type time; command -v time; command -V time"
	for _, test := range []struct {
		bash   bool
		stdout string
	}{
		{true, "keyword\ntime is a shell keyword\ntime\ntime is a shell keyword\n"},
		{false, "builtin\ntime is a builtin applet\ntime\ntime is a builtin applet\n"},
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
