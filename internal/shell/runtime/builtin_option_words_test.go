package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// A builtin busybox has names an option it cannot take as busybox's nextopt names it,
// `illegal option -Y`, as export, set and cd already did; read, unset, wait and jobs had
// their own words, read and unset with a list of what they take. declare and type's options,
// which are bash's alone, are named in bash's words, `-Y: invalid option`. Measured against
// busybox-w32 and bash.
func TestRuntime_aBuiltinNamesAnOptionItCannotTake(t *testing.T) {
	for _, test := range []struct{ script, stderr string }{
		{"read -Y x", "nemosh: line 1: read: illegal option -Y\n"},
		{"unset -Y", "nemosh: line 1: unset: illegal option -Y\n"},
		{"wait -Y", "nemosh: line 1: wait: illegal option -Y\n"},
		{"jobs -Y", "nemosh: line 1: jobs: illegal option -Y\n"},
		{"declare -Y", "nemosh: line 1: declare: -Y: invalid option\n"},
		{"type -Y ls", "nemosh: line 1: type: -Y: invalid option\n"},
	} {
		var stderr bytes.Buffer
		rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: new(bytes.Buffer), Stderr: &stderr})
		status := rt.RunScript(context.Background(), test.script+"\n")
		rt.CloseBatch(status)
		if stderr.String() != test.stderr || status == 0 {
			t.Errorf("%s: %q, status %d; want %q", test.script, stderr.String(), status, test.stderr)
		}
	}
}
