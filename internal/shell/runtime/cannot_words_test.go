package runtime_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// What busybox says it "can't" do, nemosh says it cannot, in busybox's shape otherwise: the
// operand quoted as busybox quotes it, and busybox's status. Measured against busybox-w32.
func TestRuntime_saysCannotWhereBusyboxSaysCant(t *testing.T) {
	for _, test := range []struct {
		script, stderr string
		status         int
	}{
		{"cd ./no-such-dir", "nemosh: line 2: cd: cannot cd to ./no-such-dir: No such file or directory\n", 2},
		{"echo a | sed 'b nolabel'", "sed: cannot find label for jump to 'nolabel'\n", 1},
		{"time -o ./no-such-dir/t true", "time: cannot open './no-such-dir/t': No such file or directory\n", 1},
	} {
		var stderr bytes.Buffer
		rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: new(bytes.Buffer), Stderr: &stderr})
		status := rt.RunScript(context.Background(), "cd '"+filepath.ToSlash(t.TempDir())+"'\n"+test.script+"\n")
		rt.CloseBatch(status)
		if stderr.String() != test.stderr || status != test.status {
			t.Errorf("%s: %q, status %d; want %q, status %d", test.script, stderr.String(), status, test.stderr, test.status)
		}
	}
}
