package runtime_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// hostProgram names something that is on PATH on this machine but is not a
// Nemosh applet or builtin, so `command -v` has to reach PATH to answer.
func hostProgram(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"hostname", "where", "which", "git"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	t.Skip("no non-applet program found on PATH")
	return ""
}

func TestRuntime_reportsTheResolvedPath_whenCommandVNamesAProgramOnPath(t *testing.T) {
	// `command -v` never consulted PATH, so it answered "absent" for every
	// external program while running those same programs happily.
	// Given
	program := hostProgram(t)

	// When
	status, stdout, stderr := runSetScript(t, "command -v "+program+"\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
	}
	if strings.TrimSpace(stdout) == "" {
		t.Fatalf("stdout = %q, want a path for %s", stdout, program)
	}
	if !strings.Contains(strings.ToLower(stdout), strings.ToLower(program)) {
		t.Fatalf("stdout = %q, want it to name %s", stdout, program)
	}
}

func TestRuntime_reportsTheNameAlone_whenCommandVNamesABuiltin(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "command -v cd\n")

	// Then
	if status != 0 || stdout != "cd\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "cd\n")
	}
}

func TestRuntime_reportsTheNameAlone_whenCommandVNamesAnApplet(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "command -v echo\n")

	// Then
	if status != 0 || stdout != "echo\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "echo\n")
	}
}

func TestRuntime_reportsTheNameAlone_whenCommandVNamesAFunction(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "f() { echo body; }\ncommand -v f\n")

	// Then
	if status != 0 || stdout != "f\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "f\n")
	}
}

func TestRuntime_reportsFailure_whenCommandVNamesNothingReachable(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "command -v definitely-not-a-command-anywhere\n")

	// Then
	if status != 1 || stdout != "" {
		t.Fatalf("status = %d, stdout = %q, want 1 and no output", status, stdout)
	}
}

func TestRuntime_reportsThePathAsWritten_whenCommandVNamesAPath(t *testing.T) {
	// Given
	dir := t.TempDir()
	name := "prog.exe"
	if runtime.GOOS != "windows" {
		name = "prog"
	}
	program := filepath.Join(dir, name)
	if err := os.WriteFile(program, []byte("MZ"), 0o755); err != nil {
		t.Fatalf("seed program: %v", err)
	}

	// When
	status, stdout, _ := runSetScript(t, "command -v "+filepath.ToSlash(program)+"\n")

	// Then
	if status != 0 || strings.TrimSpace(stdout) == "" {
		t.Fatalf("status = %d, stdout = %q, want 0 and the path echoed back", status, stdout)
	}
}
