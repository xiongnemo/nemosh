package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode a `$((` that does not close as arithmetic is a command substitution whose command
// begins with a subshell, as bash reads it: `$((which x || echo y)2>/dev/null)`, the gnunet-gtk
// package's, and `$(( echo 1 ... ) )` over several lines. busybox calls it an unterminated
// arithmetic expansion and runs none of the script, which nemosh and sh keep. Measured
// 2026-11-22 with Git for Windows' bash 5.3; Oils' paren-ambiguity.test.sh.
func TestArithFallback_bashModeIsACommandSubstitution(t *testing.T) {
	for _, test := range []struct {
		code, stdout string
		status       int
		bash         bool
	}{
		{code: "x=\"$((nosuchcmd-zz 2>/dev/null || echo /opt/gnome/bin/x)2>/dev/null)\"\necho \"$x\"\necho bye", stdout: "/opt/gnome/bin/x\nbye\n", bash: true},
		{code: "echo $(( echo 1\necho 2\n(( x ))\n: $(( x ))\necho 3\n) )", stdout: "1 2 3\n", bash: true},
		{code: "echo $(( 1 + 2 )) $((3*4))", stdout: "3 12\n", bash: true},
		{code: "shopt -s expand_aliases; alias e='echo al'\nx=$((echo hi)2>/dev/null); e \"$x\"\ne two", stdout: "al hi\nal two\n", bash: true},
		{code: "x=\"$((nosuchcmd-zz 2>/dev/null || echo /opt/gnome/bin/x)2>/dev/null)\"\necho \"$x\"", status: 2},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		status := rt.RunScript(context.Background(), test.code+"\n")
		if status != test.status || stdout.String() != test.stdout {
			t.Errorf("bash mode %v, %q: status %d, stdout %q, stderr %q; want %d and %q", test.bash, test.code, status, stdout.String(), stderr.String(), test.status, test.stdout)
		}
	}
}
