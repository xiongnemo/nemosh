package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A compound command whose redirection's word fails to expand is answered as the error is,
// and no file is opened: `{ echo in; } > ${x?oops}` says `x: oops` and ends the script, in
// busybox-w32 and bash alike. It went on to open the empty path the failed word left, and said
// `cannot create : nonexistent directory` after the error.
func TestRuntime_compoundRedirectionWordThatFailsOpensNothing(t *testing.T) {
	for _, bash := range []bool{false, true} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if bash {
			rt.EnterBashMode()
		}
		status := rt.RunScript(context.Background(), "{ echo in; } > ${x?oops}; echo same\necho next\n")
		if stdout.Len() != 0 || !strings.HasSuffix(stderr.String(), "x: oops\n") || strings.Count(stderr.String(), "\n") != 1 || status == 0 {
			t.Errorf("bash mode %v: status %d, stdout %q, stderr %q; want the error alone, and the script ended", bash, status, stdout.String(), stderr.String())
		}
	}
}
