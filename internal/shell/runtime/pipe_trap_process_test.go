package runtime

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"testing"
)

// A nemosh whose script ignores SIGPIPE, or catches it, goes on past a write into its own
// standard output once the reader has gone, which off Windows takes the process's SIGPIPE:
// Go's runtime ends a process at the first such write otherwise. One that does neither ends
// there, with SIGPIPE's 141, and so does one that put the default back.
func TestPipeTrap_theProcessKeepsWhatItsShellMadeOfSIGPIPE(t *testing.T) {
	binary := os.Getenv(jobBinaryVariable)
	if binary == "" {
		t.Skip("no nemosh was built to run")
	}
	for _, test := range []struct {
		name, script, stderr string
		status               int
	}{
		{name: "ignored", script: "trap '' PIPE\necho one\necho \"after $?\" >&2\n", stderr: "nemosh: line 2: echo: write error: Broken pipe\nafter 1\n"},
		{name: "caught", script: "trap 'echo got >&2' PIPE\necho one\necho \"after $?\" >&2\n", stderr: "nemosh: line 2: echo: write error: Broken pipe\ngot\nafter 1\n"},
		{name: "the default", script: "echo one\necho after >&2\n", status: brokenPipeStatus},
		{name: "the default again", script: "trap '' PIPE\ntrap - PIPE\necho one\necho after >&2\n", status: brokenPipeStatus},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			reader.Close()
			var stderr bytes.Buffer
			command := exec.Command(binary, "-c", test.script)
			command.Stdout, command.Stderr = writer, &stderr

			// When
			err = command.Run()
			writer.Close()

			// Then
			if _, exited := errors.AsType[*exec.ExitError](err); err != nil && !exited {
				t.Fatal(err)
			}
			status, _ := processOutcome(command.ProcessState)
			if status != test.status || stderr.String() != test.stderr {
				t.Fatalf("status %d, stderr %q, want %d and %q", status, stderr.String(), test.status, test.stderr)
			}
		})
	}
}
