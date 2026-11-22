package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Under set -u a name arithmetic reads that is not set is bash's unbound variable in bash mode,
// in $((...)), ((...)) and an array's subscript alike, and ends the script as any is: 1, and 127
// from a -c command string. A branch arithmetic does not evaluate reads nothing. busybox takes
// such a name for 0, as nemosh and sh do. Measured 2026-11-22 with Git for Windows' bash 5.3
// and busybox-w32; Oils' arith.test.sh, nounset cases.
func TestArithNounset_bashModeIsUnbound(t *testing.T) {
	for _, test := range []struct {
		code, stdout, said string
		status             int
		bash               bool
	}{
		{code: "set -u; echo $(( x + 1 )); echo after", said: "x: unbound variable\n", status: 1, bash: true},
		{code: "set -u; (( y++ )); echo after", said: "y: unbound variable\n", status: 1, bash: true},
		{code: "set -u; echo $(( a[0] + 1 )); echo after", said: "a: unbound variable\n", status: 1, bash: true},
		{code: "set -u; z=5; echo $(( z + 1 )); echo $(( 1 || nope )); echo $(( 0 && nope ))", stdout: "6\n1\n0\n", bash: true},
		{code: "set -u; echo $(( x + 1 )); echo after", stdout: "1\nafter\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		status := rt.RunScript(context.Background(), test.code+"\n")
		if status != test.status || stdout.String() != test.stdout || !strings.HasSuffix(stderr.String(), test.said) || test.said == "" && stderr.Len() != 0 {
			t.Errorf("bash mode %v, %s: status %d, stdout %q, stderr %q; want %d, %q and %q", test.bash, test.code, status, stdout.String(), stderr.String(), test.status, test.stdout, test.said)
		}
	}
}
