package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// set lists no environment entry whose name is no variable's: Windows keeps `=ExitCode`, `=C:` and
// `ProgramFiles(x86)` in a process's environment, which a child is still handed, and set listed
// the first as `='ExitCode=00000000'` and the last with its value. Neither busybox-w32 nor bash
// lists them so; both list the variables a script can name.
func TestSetListing_leavesOutNamesNoVariableHas(t *testing.T) {
	environment := newHostEnvironment([]string{"=ExitCode=00000000", "=C:=C:\\work", "ProgramFiles(x86)=C:\\Program Files (x86)", "PLAIN=yes"})
	for _, bash := range []bool{false, true} {
		var stdout, stderr bytes.Buffer
		rt := NewWithState(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr}, State{Cwd: WorkingDirectory(t.TempDir()), Env: environment})
		if bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), "set\nenv | grep -c 'ProgramFiles(x86)'\n")
		listing := "\n" + stdout.String()
		if strings.Contains(listing, "\n=") || strings.Contains(listing, "\nProgramFiles(x86)=") || !strings.Contains(listing, "\nPLAIN=") || !strings.HasSuffix(listing, "\n1\n") {
			t.Errorf("bash mode %v: listing %q; want PLAIN and neither =... nor ProgramFiles(x86), which env still has", bash, stdout.String())
		}
	}
}
