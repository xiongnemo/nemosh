package runtime_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// **Backgrounding a command says which job it became, and how to stop it.**
//
// busybox answers `[1] 19676` -- a job number and a process id. The number here is real
// and the pid is not: a background job in this shell is a goroutine, which is why `$!`
// already answers `%1` rather than a number (see launchBackgroundSnapshot, and the
// divergence recorded in docs/support-matrix.md). So the line carries the handle that
// actually works, and says what to do with it, rather than a pid-shaped lie.
//
// On stderr, which is where busybox puts it -- measured, with stdout redirected away, and
// it matters: on stdout it would land inside `x=$(cmd &)`.
//
// Interactive only. A script that backgrounds something wants its output, not a running
// commentary, and busybox announces nothing when it is not interactive either.

func TestBackground_announcesTheJobInteractively(t *testing.T) {
	registry := applets.NewRegistry(backgroundApplet{name: "worker", run: func(_ context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		return nil
	}})
	var stdout, stderr bytes.Buffer
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	script, err := runtime.ParseScript("worker &\nwait\n")
	if err != nil {
		t.Fatal(err)
	}
	rt.RunInteractive(context.Background(), script)

	announcement := stderr.String()
	if !strings.Contains(announcement, "[1]") {
		t.Errorf("stderr = %q, want the job number in it", announcement)
	}
	if !strings.Contains(announcement, "kill %1") {
		t.Errorf("stderr = %q, want it to say how to stop the job -- `%%1` is the handle here, "+
			"and someone who has only ever killed a pid has no reason to guess it", announcement)
	}
	// Not on stdout, which is what keeps `x=$(worker &)` clean.
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing: the announcement belongs on stderr", stdout.String())
	}
}

func TestBackground_saysNothingInAScript(t *testing.T) {
	registry := applets.NewRegistry(backgroundApplet{name: "worker", run: func(_ context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		return nil
	}})
	var stdout, stderr bytes.Buffer
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	if status := rt.RunScript(context.Background(), "worker &\nwait\n"); status != 0 {
		t.Fatalf("status = %d", status)
	}
	if stderr.Len() != 0 || stdout.Len() != 0 {
		t.Errorf("a script announced the job: stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

// TestBackground_announcementStaysOutOfCommandSubstitution is the failure mode that would
// make this worse than saying nothing: a captured value with a job line glued to it.
//
// The value is reported by a second applet in this registry rather than by `echo`. The
// first version used `echo`, which is not in a registry built for a test, so it went out
// to PATH -- found here, where scoop has installed a shim for every applet, and not on a
// Windows runner. Green locally and red on CI, which is the trap AGENTS.md already names:
// a test must not sample the machine it runs on.
func TestBackground_announcementStaysOutOfCommandSubstitution(t *testing.T) {
	registry := applets.NewRegistry(
		backgroundApplet{name: "worker", run: func(_ context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
			return nil
		}},
		backgroundApplet{name: "report", run: func(_ context.Context, args []string, _ io.Reader, stdout io.Writer, _ io.Writer) error {
			_, err := io.WriteString(stdout, "["+strings.Join(args, " ")+"]\n")
			return err
		}},
	)
	var stdout, stderr bytes.Buffer
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	script, err := runtime.ParseScript("captured=$(worker &)\nreport \"$captured\"\n")
	if err != nil {
		t.Fatal(err)
	}
	rt.RunInteractive(context.Background(), script)

	if got := stdout.String(); got != "[]\n" {
		t.Errorf("captured %q, want the substitution to be empty -- the announcement must not "+
			"be part of what a command substitution collects", got)
	}
	// And it did go somewhere, so this is not passing because nothing was announced.
	if !strings.Contains(stderr.String(), "[1]") {
		t.Errorf("stderr = %q, want the announcement to have been made on stderr", stderr.String())
	}
}
