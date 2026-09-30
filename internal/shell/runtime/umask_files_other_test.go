//go:build !windows

package runtime

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// The shell's umask is what the files it makes are made through: by a redirection, by an
// applet, by a child, and by a job, and a subshell's own mask reaches the files the subshell
// makes. `umask 077` changed only what `umask` printed, and every file was made through the
// mask the process started with. This shell shares the test's process, whose mask is not its
// to set: the modes come from the mode each file asks for, and a child's from the process's
// mask set to the shell's while it forks, and the test's own mask is as it was after.
func TestUmask_reachesTheFilesTheShellMakes(t *testing.T) {
	// Given
	dir := t.TempDir()
	before := syscall.Umask(0)
	syscall.Umask(before)
	if before&^0o022 != 0 {
		t.Skipf("the test's umask, %04o, clears more than 022, and would clear bits these masks leave", before)
	}
	script := "cd '" + filepath.ToSlash(dir) + "'\n" +
		"umask 077\n" +
		": > redirect; touch touched; mkdir made; echo x | tee teed > /dev/null\n" +
		"/bin/sh -c ': > child'\n" +
		"{ : > job; } & wait\n" +
		"(umask 027; : > sub_redirect; touch sub_touched; mkdir sub_made; /bin/sh -c ': > sub_child')\n" +
		"x=$(umask 062; : > substituted)\n"
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), script)
	rt.CloseBatch(status)

	// Then
	after := syscall.Umask(0)
	syscall.Umask(after)
	if status != 0 {
		t.Fatalf("status %d, stderr %q", status, stderr.String())
	}
	assertModes(t, dir, map[string]os.FileMode{
		"redirect": 0o600, "touched": 0o600, "made": 0o700, "teed": 0o600, "child": 0o600, "job": 0o600,
		"sub_redirect": 0o640, "sub_touched": 0o640, "sub_made": 0o750, "sub_child": 0o640,
		"substituted": 0o604,
	})
	if after != before {
		t.Fatalf("the test's umask was %04o and is %04o", before, after)
	}
}

// nemosh owns its process's mask, so `umask` sets that too, and a mask that clears less than
// the one the process started with reaches its files as well: `umask 0002` makes a file 0664,
// as the Oils case has it. Here nemosh is the binary the tests built, run as a child.
func TestUmask_isTheProcesssInNemosh(t *testing.T) {
	binary := os.Getenv(jobBinaryVariable)
	if binary == "" {
		t.Skip("no nemosh was built, so there is no process of its own to look at")
	}
	// Given
	dir := t.TempDir()
	script := "umask 0002\n" +
		": > redirect; touch touched; mkdir made; /bin/sh -c ': > child'\n" +
		"umask 027; { : > job; } & wait\n" +
		"umask 0; : > open\n"
	command := exec.Command(binary, "-c", script)
	command.Dir = dir

	// When
	output, err := command.CombinedOutput()

	// Then
	if err != nil {
		t.Fatalf("nemosh -c: %v, output %q", err, output)
	}
	assertModes(t, dir, map[string]os.FileMode{
		"redirect": 0o664, "touched": 0o664, "made": 0o775, "child": 0o664, "job": 0o640, "open": 0o666,
	})
}

func assertModes(t *testing.T, dir string, want map[string]os.FileMode) {
	t.Helper()
	for name, mode := range want {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if got := info.Mode().Perm(); got != mode {
			t.Errorf("%s is %04o, want %04o", name, got, mode)
		}
	}
}
