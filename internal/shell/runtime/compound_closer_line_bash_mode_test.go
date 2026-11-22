package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode a compound command's redirection that fails is said on the line it is written
// on, its closer's, `done > f` and `} > f`, and a function body's on the line its definition
// begins on, as bash says them. nemosh and sh keep the lines they said, near where the command
// begins, as busybox says them. Measured 2026-11-22 with Git for Windows' bash 5.3.
func TestCompoundCloserLine_bashModeIsTheClosers(t *testing.T) {
	script := strings.Join([]string{
		"for i in 1; do", "  :", "done > /no/such/dir/a",
		"while false; do", "  :", "done < /no/such/b",
		"if true; then", "  :", "fi 2> /no/such/dir/c",
		"{", "  :", "} > /no/such/dir/d",
		"case x in", "  x) : ;;", "esac > /no/such/dir/e",
		"(", "  :", ") > /no/such/dir/f",
		"g() {", "  :", "} > /no/such/dir/h",
		"g",
	}, "\n") + "\n"
	for _, test := range []struct {
		bash  bool
		lines []string
	}{
		{true, []string{"3", "6", "9", "12", "15", "18", "19"}},
		{false, []string{"1", "4", "7", "11", "13", "17", "20"}},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), script)
		var got []string
		for _, said := range strings.Split(strings.TrimSpace(stderr.String()), "\n") {
			line, _, _ := strings.Cut(strings.TrimPrefix(said, "nemosh: line "), ":")
			got = append(got, line)
		}
		if strings.Join(got, " ") != strings.Join(test.lines, " ") {
			t.Errorf("bash mode %v: lines %v, want %v\n%s", test.bash, got, test.lines, stderr.String())
		}
	}
}
