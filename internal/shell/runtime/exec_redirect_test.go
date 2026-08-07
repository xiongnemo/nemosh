package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// runExecScript closes the shell afterwards, the way cmd/nemosh does. A
// descriptor opened by `exec` is owned by the shell rather than by a command,
// so nothing before that point can release it.
func runExecScript(t *testing.T, source string) (int, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr},
		runtime.State{Cwd: runtime.WorkingDirectory(dir)})
	status := rt.RunScript(context.Background(), source)
	rt.CloseBatch(status)
	return status, stdout.String(), stderr.String(), dir
}

func TestRuntime_redirectsTheShellsOwnOutput_whenExecHasNoCommand(t *testing.T) {
	// `exec > file` reported success, created nothing, and left output going to
	// the terminal.
	// When
	status, stdout, stderr, dir := runExecScript(t, "exec > out.txt\necho first\necho second\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want everything to have gone to the file", stdout)
	}
	content, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil {
		t.Fatalf("read redirected file: %v", err)
	}
	if string(content) != "first\nsecond\n" {
		t.Fatalf("file = %q, want %q", content, "first\nsecond\n")
	}
}

func TestRuntime_redirectsTheShellsOwnInput_whenExecHasNoCommand(t *testing.T) {
	// When
	status, stdout, stderr, _ := runExecScript(t, "printf 'seeded\\n' > in.txt\nexec < in.txt\nread line\necho [$line]\n")

	// Then
	if status != 0 || stdout != "[seeded]\n" {
		t.Fatalf("status = %d, stdout = %q, stderr = %q, want 0 and %q", status, stdout, stderr, "[seeded]\n")
	}
}

func TestRuntime_keepsTheRedirect_whenExecIsFollowedByMoreCommands(t *testing.T) {
	// The point of a redirect-only exec is that it outlives the command it was
	// written on.
	// When
	status, _, _, dir := runExecScript(t, "exec > out.txt\nfor i in 1 2 3; do echo $i; done\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	content, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil {
		t.Fatalf("read redirected file: %v", err)
	}
	if string(content) != "1\n2\n3\n" {
		t.Fatalf("file = %q, want %q", content, "1\n2\n3\n")
	}
}

func TestRuntime_appendsToTheFile_whenExecUsesTheAppendOperator(t *testing.T) {
	// When
	status, _, _, dir := runExecScript(t, "printf 'kept\\n' > out.txt\nexec >> out.txt\necho added\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	content, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil {
		t.Fatalf("read redirected file: %v", err)
	}
	if string(content) != "kept\nadded\n" {
		t.Fatalf("file = %q, want %q", content, "kept\nadded\n")
	}
}

func TestRuntime_reportsTheFailure_whenAnExecRedirectCannotOpen(t *testing.T) {
	// When
	status, _, stderr, _ := runExecScript(t, "exec > nosuchdir/out.txt\n")

	// Then
	if status != 1 {
		t.Fatalf("status = %d, want 1", status)
	}
	if !strings.Contains(stderr, "nosuchdir") {
		t.Fatalf("stderr = %q, want it to name the path that could not be opened", stderr)
	}
}

func TestRuntime_leavesTheShellRunning_whenExecOnlyRedirects(t *testing.T) {
	// With a command, exec replaces the shell; with only redirections it does
	// not (POSIX 2.14).
	// When
	status, stdout, _, _ := runExecScript(t, "exec 2> err.txt\necho still-here\n")

	// Then
	if status != 0 || stdout != "still-here\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "still-here\n")
	}
}
