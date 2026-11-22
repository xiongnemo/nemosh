package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode a here-document's word may hold a command substitution or an arithmetic
// expansion, `cat <<$(a)`, `cat <<x$(a b)y`, `cat <<$((1+2))`, taken as written for the line
// that ends the body, as bash takes it. busybox's word ends at the `(`, a syntax error, which
// nemosh and sh keep. Measured 2026-11-22 with Git for Windows' bash 5.3; Oils' here-doc.test.sh.
func TestHeredocWord_bashModeHoldsASubstitution(t *testing.T) {
	for _, test := range []struct {
		code, stdout string
		status       int
		bash         bool
	}{
		{code: "cat <<$(a)\nhere\n$(a)\necho st=$?", stdout: "here\nst=0\n", bash: true},
		{code: "cat <<x$(a b)y\nhere\nx$(a b)y\necho st=$?", stdout: "here\nst=0\n", bash: true},
		{code: "cat <<$((1+2))\nhere\n$((1+2))\necho st=$?", stdout: "here\nst=0\n", bash: true},
		{code: "cat <<a$(( 1 + 2 ))\nhere\na$(( 1 + 2 ))", stdout: "here\n", bash: true},
		{code: "x=1; cat <<\"$(a)\"\nhere $x\n$(a)", stdout: "here $x\n", bash: true},
		{code: "x=1; cat <<$(a)\nhere $x\n$(a)", stdout: "here 1\n", bash: true},
		{code: "cat <<$(a)\nhere\n$(a)\necho st=$?", status: 2},
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
