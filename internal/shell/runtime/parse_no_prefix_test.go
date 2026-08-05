package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestRuntime_RunScript_hasNoPrefixEffects_whenInputIsIncomplete(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		check  func(*testing.T, Runtime, string, *bytes.Buffer)
	}{
		{name: "output", prefix: "echo prefix\n", check: checkNoPrefixOutput},
		{name: "assignment", prefix: "VALUE=changed\n", check: checkNoPrefixAssignment},
		{name: "cwd", prefix: "cd child\n", check: checkNoPrefixCWD},
		{name: "trap", prefix: "trap 'echo trapped' EXIT\n", check: checkNoPrefixOutput},
		{name: "redirect file", prefix: "echo created > created.txt\n", check: checkNoPrefixRedirect},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "child"), 0o700); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			rt := NewWithState(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr}, State{Cwd: WorkingDirectory(dir)})

			status := rt.RunScript(context.Background(), test.prefix+"if true\n")

			if status != 2 {
				t.Fatalf("RunScript() status = %d, want 2", status)
			}
			test.check(t, rt, dir, &stdout)
		})
	}
}

func checkNoPrefixOutput(t *testing.T, _ Runtime, _ string, stdout *bytes.Buffer) {
	t.Helper()
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func checkNoPrefixAssignment(t *testing.T, rt Runtime, _ string, _ *bytes.Buffer) {
	t.Helper()
	if _, exists := rt.vars["VALUE"]; exists {
		t.Fatal("VALUE was assigned by invalid script prefix")
	}
}

func checkNoPrefixCWD(t *testing.T, rt Runtime, dir string, _ *bytes.Buffer) {
	t.Helper()
	if rt.WorkingDirectory() != filepathDisplay(dir) {
		t.Fatalf("cwd = %q, want %q", rt.WorkingDirectory(), filepathDisplay(dir))
	}
}

func checkNoPrefixRedirect(t *testing.T, _ Runtime, dir string, _ *bytes.Buffer) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, "created.txt")); !os.IsNotExist(err) {
		t.Fatalf("redirect file exists or stat failed: %v", err)
	}
}
