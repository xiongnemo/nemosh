package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_returnsFromDotScriptWithStatus_whenReturnRuns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	scriptPath := writeReturnScript(t, "echo before\nreturn 7\necho unreachable\n")
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), ". "+scriptPath+"\necho after\n")

	// Then
	if status != 0 {
		t.Fatalf("expected final status 0, got %d", status)
	}
	if got := stdout.String(); got != "before\nafter\n" {
		t.Fatalf("expected return output %q, got %q", "before\nafter\n", got)
	}
}

func TestRuntime_usesReturnStatusForDotCommand_whenReturnRunsInOrList(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	scriptPath := writeReturnScript(t, "return 7\necho unreachable\n")
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), ". "+scriptPath+" || echo recovered\n")

	// Then
	if status != 0 {
		t.Fatalf("expected final status 0, got %d", status)
	}
	if got := stdout.String(); got != "recovered\n" {
		t.Fatalf("expected recovered output %q, got %q", "recovered\n", got)
	}
}

// Outside a function and a sourced file, return ends the shell as exit does, as busybox-w32
// reads it: the script, a subshell or a command substitution ends with its status, and the
// EXIT trap sees that status. It was reported and the script went on, which neither reference
// does: bash reports it and goes on with status 2.
func TestRuntime_endsTopLevelScript_whenReturnRunsOutsideAFunction(t *testing.T) {
	for index, test := range []struct {
		script, want string
		status       int
	}{
		{"return 7\necho after\n", "", 7},
		{"trap 'echo trap=$?' EXIT\nreturn 3\necho no\n", "trap=3\n", 3},
		{"for i in 1 2; do return 5; done\necho after\n", "", 5},
		{"(return 6; echo no); echo sub=$?\nx=$(return 4; echo no); echo cs=$? \"[$x]\"\n", "sub=6\ncs=4 []\n", 0},
		{"f() { (return 2); echo inner=$?; return 9; }\nf; echo f=$?\n", "inner=2\nf=9\n", 0},
	} {
		var stdout, stderr bytes.Buffer
		rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
		status := rt.RunScript(context.Background(), test.script)
		rt.CloseBatch(status)
		if status != test.status || stdout.String() != test.want || stderr.Len() != 0 {
			t.Errorf("%d: %q: got %q/%d, stderr %q; want %q/%d as busybox-w32 answers",
				index, test.script, stdout.String(), status, stderr.String(), test.want, test.status)
		}
	}
}

// In a trap's action at the top level, return is only reported and the script goes on, as bash
// has it: `trap 'return 42' DEBUG` is bash's alone, busybox-w32 having no DEBUG trap.
func TestRuntime_reportsReturnInATopLevelTrapActionAndGoesOn(t *testing.T) {
	stdout, status := runScriptCapturing("trap 'return 42' DEBUG\necho A\necho B\n")
	if stdout != "A\nB\n" || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as bash answers", stdout, status, "A\nB\n")
	}
}

func writeReturnScript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.ToSlash(filepath.Join(dir, "library.sh"))
	if err := os.WriteFile(scriptPath, []byte(content), 0o600); err != nil {
		t.Fatalf("expected return fixture write to succeed, got %v", err)
	}
	return scriptPath
}
