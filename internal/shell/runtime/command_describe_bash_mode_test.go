package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// command -v and -V in bash mode describe every name, as bash's do, and are 0 when any was
// found, 1 when none; an alias is quoted as bash's alias listing quotes it. busybox's describe
// the first name only, which nemosh and sh keep. Measured 2026-11-22 with Git for Windows' bash
// 5.3; Oils' builtin-meta.test.sh, command -v with multiple names.
func TestCommandDescribe_bashModeDescribesEveryName(t *testing.T) {
	script := "myfunc() { echo x; }; alias a=\"it's\"\n" +
		"command -v echo myfunc ZZZ for a; echo status=$?\n" +
		"command -v ZZZ echo; echo status=$?\n" +
		"command -v ZZZ; echo status=$?\n" +
		"command -V echo ZZZ; echo status=$?\n"
	for _, test := range []struct {
		bash bool
		want string
	}{
		{bash: true, want: "echo\nmyfunc\nfor\nalias a='it'\\''s'\nstatus=0\necho\nstatus=0\nstatus=1\necho is a shell builtin\nstatus=0\n"},
		{bash: false, want: "echo\nstatus=0\nstatus=127\nstatus=127\necho is a shell builtin\nstatus=0\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), script)
		if stdout.String() != test.want {
			t.Errorf("bash mode %v: stdout %q, want %q; stderr %q", test.bash, stdout.String(), test.want, stderr.String())
		}
	}
}
