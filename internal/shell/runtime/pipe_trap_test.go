package runtime_test

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// `trap "" PIPE`: the shell's own write into a pipe no one reads fails and says so, and the
// stage goes on, as bash's and ash_test echo_write_error's; a program's write fails as one
// that inherited the ignored signal does. A trap for it is dropped in the stage, as in a
// subshell, which ends as SIGPIPE ends it. Without the trap the stage ends at the write.
func TestPipeTrap_aPipeStageWithSIGPIPEIgnoredGoesOn(t *testing.T) {
	for _, test := range []struct{ name, script, stdout, stderr string }{
		{
			name:   "the shell's own write",
			script: "trap '' PIPE\n{ while echo x; do :; done; echo \"stopped $?\" >&2; } | head -1\n",
			stdout: "x\n", stderr: "echo: write error: Broken pipe\nstopped 0\n",
		},
		{
			name:   "a program's",
			script: "trap '' 13\n{ yes; echo \"yes $?\" >&2; } | head -1\n",
			stdout: "y\n", stderr: "yes: write error: Broken pipe\nyes 1\n",
		},
		{
			name:   "a trap is dropped in the stage",
			script: "trap 'echo got >&2' PIPE\n{ while echo x; do :; done; echo stopped >&2; } | head -1\necho end\n",
			stdout: "x\nend\n",
		},
		{
			name:   "the default",
			script: "{ while echo x; do :; done; echo stopped >&2; } | head -1\n{ yes; echo \"yes $?\" >&2; } | head -1\n",
			stdout: "x\ny\n", stderr: "yes 141\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 || stdout != test.stdout || stderr != test.stderr {
				t.Fatalf("status %d, stdout %q, stderr %q, want 0, %q and %q", status, stdout, stderr, test.stdout, test.stderr)
			}
		})
	}
}

// A trap for SIGPIPE runs once the command whose write found no reader has finished, after the
// write has failed and said so, and the shell goes on, as bash's does.
func TestPipeTrap_aCaughtSIGPIPERunsTheTrapAfterTheFailedWrite(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	reader.Close()
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: writer, Stderr: &stderr})
	done := make(chan int, 1)

	// When
	go func() {
		done <- rt.RunScript(context.Background(), "trap 'echo got >&2' PIPE\necho one\necho \"after $?\" >&2\n")
	}()

	// Then
	select {
	case status := <-done:
		if want := "echo: write error: Broken pipe\ngot\nafter 1\n"; status != 0 || stderr.String() != want {
			t.Fatalf("status %d, stderr %q, want 0 and %q", status, stderr.String(), want)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the shell did not finish")
	}
}

// The trap is listed, by name, as any other is, and `13` is its number.
func TestPipeTrap_isListed(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "trap '' PIPE\ntrap\ntrap -p PIPE\ntrap - 13\ntrap\necho end\n")

	// Then
	if want := "trap -- '' PIPE\ntrap -- '' PIPE\nend\n"; status != 0 || stdout != want || stderr != "" {
		t.Fatalf("status %d, stdout %q, stderr %q, want 0 and %q", status, stdout, stderr, want)
	}
}
