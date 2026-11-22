package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode `$@` and `$*` are unset when there are no positional parameters, for `-` and `+`:
// `${@-minus}` is minus and `${@+plus}` nothing, quoted or not. busybox takes them as always set,
// and so do nemosh and sh. One empty parameter is set in both. Measured 2026-11-22 with Git for
// Windows' bash 5.3; Oils' var-op-test.test.sh.
func TestEmptyPositional_bashModeIsUnset(t *testing.T) {
	const code = "set --\necho \"a=${@-minus}\" \"b=${@+plus}\" c=${*-star} \"d=${*+plus}\"\nx=${@-assigned}; echo \"x=$x\"\nset -- ''\necho \"e=${@-minus}\" \"f=${@+plus}\""
	for _, test := range []struct {
		bash   bool
		stdout string
	}{
		{true, "a=minus b= c=star d=\nx=assigned\ne= f=plus\n"},
		{false, "a= b=plus c= d=plus\nx=\ne= f=plus\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		status := rt.RunScript(context.Background(), code+"\n")
		if status != 0 || stdout.String() != test.stdout {
			t.Errorf("bash mode %v: status %d, stdout %q, stderr %q; want %q", test.bash, status, stdout.String(), stderr.String(), test.stdout)
		}
	}
}
