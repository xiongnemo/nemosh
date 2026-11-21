package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// bash's eval takes -- as the end of its options, so `eval -- "$code"` runs the code, as
// starship's init and atuin's widgets write it; busybox's runs a command named --, status 127.
// bash mode takes bash's, and nemosh and sh keep busybox's. Measured 2026-11-21 with Git for
// Windows' bash 5.3, --posix as well, and busybox-w32.
func TestEval_bashModeEndsOptionsAtDoubleDash(t *testing.T) {
	script := "eval -- echo hi; echo status=$?\n"
	for _, test := range []struct {
		name         string
		bash         bool
		stdout, fail string
	}{
		{name: "nemosh", stdout: "status=127\n", fail: "--: not found"},
		{name: "bash mode", bash: true, stdout: "hi\nstatus=0\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
			if test.bash {
				rt.EnterBashMode()
			}
			rt.RunScript(context.Background(), script)
			said := stderr.String()
			if stdout.String() != test.stdout || (test.fail == "" && said != "") || (test.fail != "" && !strings.Contains(said, test.fail)) {
				t.Fatalf("stdout %q, stderr %q", stdout.String(), stderr.String())
			}
		})
	}
}
