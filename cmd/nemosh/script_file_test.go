package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type runResult struct {
	stdout string
	stderr string
	status int
	err    error
}

func runArgs(t *testing.T, dir string, args ...string) runResult {
	t.Helper()
	if dir != "" {
		t.Chdir(dir)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := command{stdin: &bytes.Buffer{}, stdout: &stdout, stderr: &stderr}
	err := cmd.run(context.Background(), args)
	result := runResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
	if status, ok := err.(exitStatus); ok {
		result.status = int(status)
	} else if err != nil {
		result.status = -1
	}
	return result
}

func writeScript(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return dir
}

func TestRun_executesAScriptFileWithItsOwnNameAndArguments(t *testing.T) {
	dir := writeScript(t, "hello.sh", "echo $0 $1 $2 $#\n")

	got := runArgs(t, dir, "nemosh", "hello.sh", "a", "b")

	if got.err != nil {
		t.Fatalf("run: %v (stderr = %q)", got.err, got.stderr)
	}
	if want := "hello.sh a b 2\n"; got.stdout != want {
		t.Fatalf("stdout = %q, want %q", got.stdout, want)
	}
}

func TestRun_scriptFileExitStatusReachesTheCaller(t *testing.T) {
	dir := writeScript(t, "fail.sh", "exit 5\n")

	got := runArgs(t, dir, "nemosh", "fail.sh")

	if got.status != 5 {
		t.Fatalf("status = %d, want 5 (err = %v, stderr = %q)", got.status, got.err, got.stderr)
	}
}

// A script that cannot be opened is a 127, the same status POSIX shells use for
// a command that does not exist.
func TestRun_missingScriptFileIsNotFound(t *testing.T) {
	got := runArgs(t, t.TempDir(), "nemosh", "no-such-script.sh")

	if got.status != 127 {
		t.Fatalf("status = %d, want 127 (err = %v)", got.status, got.err)
	}
	if !strings.Contains(got.stderr, "no-such-script.sh") {
		t.Fatalf("stderr = %q, want it to name the script", got.stderr)
	}
}

// busybox resolves `busybox cat` to the applet even when ./cat exists, and the
// same rule keeps `nemosh echo` predictable regardless of the directory.
func TestRun_appletNamesWinOverSameNamedFiles(t *testing.T) {
	dir := writeScript(t, "echo", "echo from-file\n")

	got := runArgs(t, dir, "nemosh", "echo", "from-applet")

	if got.err != nil {
		t.Fatalf("run: %v (stderr = %q)", got.err, got.stderr)
	}
	if want := "from-applet\n"; got.stdout != want {
		t.Fatalf("stdout = %q, want %q", got.stdout, want)
	}
}

// An unrecognised option used to be swallowed and stdin read instead, which made
// a typo look like a hang.
func TestRun_unknownOptionIsAUsageError(t *testing.T) {
	got := runArgs(t, "", "nemosh", "-x")

	if got.status != 2 {
		t.Fatalf("status = %d, want 2 (err = %v)", got.status, got.err)
	}
	if !strings.Contains(got.stderr, "-x") {
		t.Fatalf("stderr = %q, want it to name the option", got.stderr)
	}
}

func TestRun_commandStringTakesItsNameAndArgumentsFromTheOperands(t *testing.T) {
	got := runArgs(t, "", "nemosh", "-c", "echo $0 $1 $#", "named", "a")

	if got.err != nil {
		t.Fatalf("run: %v (stderr = %q)", got.err, got.stderr)
	}
	if want := "named a 1\n"; got.stdout != want {
		t.Fatalf("stdout = %q, want %q", got.stdout, want)
	}
}

func TestRun_commandStringWithoutAnArgumentIsAUsageError(t *testing.T) {
	got := runArgs(t, "", "nemosh", "-c")

	if got.status != 2 {
		t.Fatalf("status = %d, want 2 (err = %v)", got.status, got.err)
	}
}
