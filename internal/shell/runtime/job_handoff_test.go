package runtime

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A job started by the shell's last command runs, though the shell has exited by the time
// it starts, as busybox's does. Its state was written by a goroutine that the exit took with
// it, so the job read nothing, said "job state: unexpected end of JSON input", and ended.
func TestProcessJobs_aJobStartedLastStillRuns(t *testing.T) {
	binary := os.Getenv(jobBinaryVariable)
	if binary == "" {
		t.Skip("no nemosh was built, so there is no shell to run")
	}
	// Given
	file := filepath.ToSlash(filepath.Join(t.TempDir(), "ran"))
	var stderr bytes.Buffer
	shell := exec.Command(binary, "-c", "echo ran > '"+file+"' &")
	shell.Env = append(os.Environ(), "NEMOSH_JOBS=process")
	shell.Stderr = &stderr

	// When: the job holds the shell's stderr, so this waits for the job too.
	err := shell.Run()

	// Then
	if err != nil {
		t.Fatalf("the shell: %v; stderr %q", err, stderr.String())
	}
	data, readErr := os.ReadFile(file)
	if readErr != nil || string(data) != "ran\n" || strings.Contains(stderr.String(), "job state") {
		t.Fatalf("the job wrote %q (%v); stderr %q", data, readErr, stderr.String())
	}
}
