package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// watch's heading is busybox's: `Every 2.0s: CMD` padded to the terminal's width, the date and
// time over its last twenty columns but one, and nothing written past them. Measured against
// busybox-w32's at 80 columns, its width with no terminal.
func TestWatchHeading_isBusyboxs(t *testing.T) {
	now := time.Date(2026, 10, 5, 7, 7, 26, 0, time.Local)
	got := watchHeading(400*time.Millisecond, "echo hi", 80, now)
	want := "Every 0.4s: echo hi" + strings.Repeat(" ", 41) + "2026-10-05 07:07:26"
	if got != want || len(got) != 79 {
		t.Fatalf("watchHeading = %q (%d), want %q", got, len(got), want)
	}
	if got := watchHeading(2*time.Second, "x", 20, now); got != "Every 2.0s: x"+strings.Repeat(" ", 19) {
		t.Fatalf("too narrow for the time: %q", got)
	}
}

// watch runs its command, clearing the screen before each run, until it is interrupted: as
// shell text in a subshell, which leaves the shell's variables alone, or under -x as the words
// they are. What it refuses it says, status 1. It was not here; busybox-w32 has it.
func TestWatch_runsUntilInterrupted(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	status := rt.RunScript(context.Background(), "x=1\ntimeout 0.45 watch -t -n 0.1 'x=2; echo \"$x\"'\necho \"st=$? x=$x\"\n")
	runs := strings.Count(stdout.String(), "\033[H\033[J2\n")
	if status != 0 || runs < 2 || !strings.HasSuffix(stdout.String(), "st=124 x=1\n") || stderr.Len() > 0 {
		t.Fatalf("got %d, %d runs, %q, %q", status, runs, stdout.String(), stderr.String())
	}
	stdout.Reset()
	rt.RunScript(context.Background(), "timeout 0.25 watch -tx -n 0.1 echo a b\n")
	if !strings.HasPrefix(stdout.String(), "\033[H\033[Ja b\n") {
		t.Fatalf("watch -x wrote %q", stdout.String())
	}
	for _, test := range []struct {
		script, says string
	}{
		{script: "watch", says: "watch: expected a command to run"},
		{script: "watch -n x true", says: "watch: invalid number 'x'"},
		{script: "watch -q true", says: "watch: unknown option -- q"},
	} {
		stderr.Reset()
		if status := rt.RunScript(context.Background(), test.script+"\n"); status != 1 || !strings.Contains(stderr.String(), test.says) {
			t.Errorf("%s = %d, %q; want 1 and %q", test.script, status, stderr.String(), test.says)
		}
	}
}
