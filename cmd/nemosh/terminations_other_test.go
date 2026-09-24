//go:build !windows

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A real SIGTERM, sent to this process while `nemosh -c` runs a script, reaches the
// script's trap -- bash 5.3's answer for a script `kill` sends TERM. The signal goes out
// only once the script says it is running, which is when the notification is live; sent
// at any other moment it would end the test binary.
func TestScript_getsATermSentFromOutside(t *testing.T) {
	for _, test := range []struct {
		name, script, stdout string
		signal               int
	}{
		{name: "trapped", script: "trap 'echo got-term' TERM\n: > \"$d/ready\"\nsleep 2\necho after\n", stdout: "got-term\nafter\n"},
		{name: "untrapped", script: "trap 'echo bye' EXIT\n: > \"$d/ready\"\nsleep 5\necho after\n", stdout: "bye\n", signal: 15},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			var stdout, stderr bytes.Buffer
			cmd := command{stdin: strings.NewReader(""), stdout: &stdout, stderr: &stderr}
			go func() {
				for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
					if _, err := os.Stat(filepath.Join(directory, "ready")); err == nil {
						_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
						return
					}
				}
			}()
			err := cmd.run(context.Background(), []string{"nemosh", "-c", "d='" + directory + "'\n" + test.script})
			signal, _ := errors.AsType[signalExit](err)
			if stdout.String() != test.stdout || int(signal) != test.signal {
				t.Fatalf("stdout %q err %v stderr %q, want %q and signal %d", stdout.String(), err, stderr.String(), test.stdout, test.signal)
			}
		})
	}
}
