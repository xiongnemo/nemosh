package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A compound command's redirection that fails is said on its own line: the line the command
// begins on, as busybox-w32 says it, and as bash says it for a compound written on one line.
// It was said on the line of the command run before it, `line 1` for every one below. bash says
// the line a redirection is written on, the last for `while` over several lines and `done <
// nofile`, which is not kept; busybox's first is said there under both names. Measured
// 2026-11-22, a script per case.
func TestCompoundRedirectionLine_isTheCommandsOwn(t *testing.T) {
	script := "echo one\n" +
		"{ cat; } < nonexist\n" +
		"while false\ndo\n  :\ndone < nonexist\n" +
		"if true; then :; fi < nonexist\n"
	for _, test := range []struct {
		bash bool
		want []string
	}{
		{bash: false, want: []string{"line 2: ", "line 3: ", "line 7: "}},
		{bash: true, want: []string{"line 2: ", "line 3: ", "line 7: "}},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), script)
		lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
		if len(lines) != len(test.want) {
			t.Fatalf("bash mode %v: stderr %q; want three diagnostics", test.bash, stderr.String())
		}
		for index, line := range lines {
			if !strings.Contains(line, test.want[index]) {
				t.Errorf("bash mode %v: diagnostic %q, want it on %q", test.bash, line, test.want[index])
			}
		}
	}
}
