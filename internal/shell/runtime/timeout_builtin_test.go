package runtime_test

import (
	"testing"
	"time"
)

// timeout runs its command and ends it once SECS have passed, as busybox-w32's applet does:
// 124 then, 137 with -s KILL, the command's own status when it finishes first, and 125 for a
// bad option, signal or duration. The shell had none, and on Windows `timeout` found
// System32's timeout.exe, which refused the line and never ran the command.
func TestTimeout_endsTheCommandOnceItsTimeIsUp(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{"timeout 5 echo quick; echo st=$?", "quick\nst=0\n"},
		{"timeout 5 false; echo st=$?", "st=1\n"},
		{"timeout 0.1 sleep 10; echo st=$?", "st=124\n"},
		{"timeout -s KILL 0.1 sleep 10; echo st=$?", "st=137\n"},
		{"timeout -sKILL -k 1 0.1 sleep 10; echo st=$?", "st=137\n"},
		{"f() { sleep 10; echo not; }; timeout 0.1 f; echo st=$?", "st=124\n"},
		{"timeout -s NOPE 1 true 2>/dev/null; echo st=$?", "st=125\n"},
		{"timeout 1 2>/dev/null; echo st=$?", "st=125\n"},
		{"timeout x true 2>/dev/null; echo st=$?", "st=125\n"},
		{"timeout -z 1 true 2>/dev/null; echo st=$?", "st=125\n"},
	} {
		start := time.Now()
		stdout, status := runScriptCapturing(test.script + "\n")
		if stdout != test.want || status != 0 {
			t.Errorf("%d: %q: got %q/%d, want %q/0, as busybox-w32 answers", index, test.script, stdout, status, test.want)
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("%d: %q took %v; the timeout did not end the command", index, test.script, elapsed)
		}
	}
}
