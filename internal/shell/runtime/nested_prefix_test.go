package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A variable a function sets reaches the shell through every call with an assignment in front
// of it: `f(){ g=1; }; h(){ x=1 f; }; y=1 h` leaves g set to 1, in busybox-w32 and bash alike.
// Each such call runs in a temporary of its caller's, whose changes are merged back when it ends,
// and a change merged into a temporary was not one the next merge out saw, so g was lost.
func TestNestedPrefixCalls_keepWhatTheInnerOneSet(t *testing.T) {
	for _, bash := range []bool{false, true} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), "f(){ g=1; }; h(){ x=1 f; }; y=1 h; echo \"[$g|${x-}|${y-}]\"\n")
		if stdout.String() != "[1||]\n" {
			t.Errorf("bash mode %v: stdout %q, stderr %q; want [1||]", bash, stdout.String(), stderr.String())
		}
	}
}
