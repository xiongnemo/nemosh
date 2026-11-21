package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A double quote in $((...)) is a quote to bash, which removes it before evaluating, as it does
// in (( )), for (( )) and a subscript already here: $(("1+2"*3)) is 7, and starship's PS0 sets
// its timer with $((T="$(starship time)",0)). busybox calls any quote there a syntax error, so
// bash mode removes them and nemosh and sh keep busybox's answer. A quote inside a command
// substitution is the substitution's. Measured 2026-11-21.
func TestArithmetic_bashModeRemovesDoubleQuotes(t *testing.T) {
	script := `echo $(("1"+1)) $(("1+2"*3)) $((y="7",y+1)) $y $(( $(echo "3") + "1" ))` + "\n"
	for _, test := range []struct {
		name         string
		bash         bool
		stdout, fail string
	}{
		{name: "nemosh", fail: "arithmetic syntax error"},
		{name: "bash mode", bash: true, stdout: "2 7 8 7 4\n"},
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
