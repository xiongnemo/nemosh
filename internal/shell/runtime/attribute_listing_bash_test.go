package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// readonly and export with no names list in bash mode as bash's do: declare's lines outside its
// POSIX mode, `declare -r q="it's"`, `declare -x v="333"`, `declare -ar arr=(...)`; under it the
// builtin's own name with declare's quoting, `readonly q="it's"`, `readonly -a arr=(...)`,
// `export v="333"`, `export w`. busybox's are `readonly q='it'"'"'s'` and `export v='333'`, which
// nemosh and sh keep. Measured 2026-11-22 with Git for Windows' bash 5.3; Oils' assign-extended
// cases.
func TestAttributeListing_bashModeIsBashs(t *testing.T) {
	script := "readonly q=\"it's\"; export v=333 w; declare -a arr=(1 2); readonly arr\n"
	for _, test := range []struct {
		bash  bool
		code  string
		lines []string
	}{
		{bash: true, code: "readonly; export", lines: []string{`declare -ar arr=([0]="1" [1]="2")`, `declare -r q="it's"`, `declare -x v="333"`, `declare -x w`}},
		{bash: true, code: "set -o posix; readonly -p; export -p", lines: []string{`readonly -a arr=([0]="1" [1]="2")`, `readonly q="it's"`, `export v="333"`, `export w`}},
		{bash: false, code: "readonly; export", lines: []string{`readonly q='it'"'"'s'`, `export v='333'`, `export w`}},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), script+test.code+"\n")
		listing := "\n" + stdout.String()
		for _, line := range test.lines {
			if !strings.Contains(listing, "\n"+line+"\n") {
				t.Errorf("bash mode %v, %s: listing lacks %q:\n%s", test.bash, test.code, line, stdout.String())
			}
		}
	}
}
