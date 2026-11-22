package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode `$[expr]` is `$((expr))`, bash's older spelling of it, in a word, in double
// quotes and in a here-document's body; brackets nest in it. busybox leaves it as text, and so do
// nemosh and sh. Measured 2026-11-22 with Git for Windows' bash 5.3; Oils' arith-context.test.sh.
func TestLegacyArithmetic_bashModeIsArithmetic(t *testing.T) {
	for _, test := range []struct {
		code, stdout string
		bash         bool
	}{
		{code: "echo $[1+2] $[3 * 4]", stdout: "3 12\n", bash: true},
		{code: "x=3; echo \"$[x*2]\" $[ x - 1 ]", stdout: "6 2\n", bash: true},
		{code: "a=(1 2); echo $[a[1]+1]; b[0]=$[1+3]; echo ${b[0]}", stdout: "3\n4\n", bash: true},
		{code: "echo \"\\$[1]\" '$[1]' \\$[1]", stdout: "$[1] $[1] $[1]\n", bash: true},
		{code: "cat <<EOF\n$[2*5] '$[1]'\nEOF", stdout: "10 '1'\n", bash: true},
		{code: "echo $[(1+2)*3]; x=$[1<<3]; echo $x ${u:-$[1+2]} \"${v:-$[2+2]}\" $[ $[1] + 1 ]", stdout: "9\n8 3 4 2\n", bash: true},
		{code: "# don't\necho \"$(echo '$[1]')\" $'$[1]' $(echo $[5])", stdout: "$[1] $[1] 5\n", bash: true},
		{code: "x=4; echo $[ \"1\" + 2 ] \"a$[x]b\" \"$[\" \"]\"", stdout: "3 a4b $[ ]\n", bash: true},
		{code: "echo $[1+2] \"$[3]\"", stdout: "$[1+2] $[3]\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		status := rt.RunScript(context.Background(), test.code+"\n")
		if status != 0 || stdout.String() != test.stdout || stderr.Len() != 0 {
			t.Errorf("bash mode %v, %q: status %d, stdout %q, stderr %q; want 0 and %q", test.bash, test.code, status, stdout.String(), stderr.String(), test.stdout)
		}
	}
}
