package runtime_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// A FILE an applet cannot open is named in the shape busybox's names it in for that applet:
// sed and awk read theirs through fopen_or_warn, which names it bare, sed going on to the
// next and awk stopping; dos2unix through xfopen_for_read, which quotes it. sed's and awk's
// said `cannot open 'f'`, and dos2unix's the name bare. awk's status is not asserted: busybox
// answers 1, and nemosh gawk's 2, which is left for a decision. Measured against busybox-w32.
func TestRuntime_namesAFileItCannotOpenAsBusyboxDoes(t *testing.T) {
	for _, test := range []struct{ script, stdout, stderr string }{
		{"echo x > a; sed p nofile a", "x\nx\n", "sed: nofile: No such file or directory\n"},
		{"echo x > a; awk 1 a nofile", "x\n", "awk: nofile: No such file or directory\n"},
		{"dos2unix nofile", "", "dos2unix: cannot open 'nofile': No such file or directory\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
		status := rt.RunScript(context.Background(), "cd '"+filepath.ToSlash(t.TempDir())+"'\n"+test.script+"\n")
		rt.CloseBatch(status)
		if stdout.String() != test.stdout || stderr.String() != test.stderr || status == 0 {
			t.Errorf("%s = %q, %q, status %d; want %q, %q", test.script, stdout.String(), stderr.String(), status, test.stdout, test.stderr)
		}
	}
}
