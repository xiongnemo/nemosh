package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// bash's export takes -a and -A: a name is exported as any is, an =(...) assigns an array, and
// with no names the exported arrays of the kind are listed; an array never reaches the
// environment. busybox calls either an illegal option and ends the script, which nemosh and sh
// keep. mise's bash hook runs `export -a chpwd_functions`, and the rest of its file was lost.
// Measured in bash 5.3, 2026-11-21.
func TestExport_bashModeTakesArrayOptions(t *testing.T) {
	script := "export -a arr\nexport -a arr2=(1 2)\ndeclare -p arr arr2\nexport -a\necho after\n"
	for _, test := range []struct {
		name         string
		bash         bool
		stdout, fail string
	}{
		{name: "nemosh", fail: "export: illegal option -a"},
		{name: "bash mode", bash: true,
			stdout: "declare -x arr\ndeclare -ax arr2=([0]=\"1\" [1]=\"2\")\ndeclare -ax arr2=([0]=\"1\" [1]=\"2\")\nafter\n"},
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
				t.Fatalf("stdout %q, stderr %q", stdout.String(), said)
			}
		})
	}
}
