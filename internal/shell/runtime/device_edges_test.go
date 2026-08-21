package runtime

import (
	"strings"
	"testing"
)

// The edges: naming a device where a path is expected, and the places that must keep refusing.
//
// Stage 4 of docs/design/device-filesystem.md, with one departure from the plan recorded here
// because it is a behaviour decision rather than an implementation detail. The plan said "decide:
// `cd /dev` succeeds". It does not, and the reason is that a working directory needs a native form:
// launching a child process sets one, and `/dev` has none. A `cd` that succeeded would leave every
// external command running in the directory the shell happened to be in before, while `pwd` said
// `/dev` -- a silent disagreement between two answers, which is worse than a refusal.
//
// `/tmp` is the contrast that makes the rule clear rather than arbitrary: `cd /tmp` works because
// /tmp *has* a native mapping, so `pwd` says /tmp and `cmd.exe` agrees.

func TestDeviceEdges_cdRefusesWithAReason(t *testing.T) {
	_, stderr, status := runScriptForDevices(t, "cd /dev\n")
	if status == 0 {
		t.Fatal("cd /dev succeeded; a device directory cannot be a working directory")
	}
	// The old message said "not a directory", which is now false: test -d /dev is true. A
	// refusal that contradicts another answer from the same shell is worse than the refusal.
	if strings.Contains(stderr, "not a directory") {
		t.Fatalf("cd /dev said %q, which contradicts test -d /dev", strings.TrimSpace(stderr))
	}
	if !strings.Contains(stderr, "device") {
		t.Fatalf("cd /dev said %q, which does not say why", strings.TrimSpace(stderr))
	}
}

// realpath answers for a device rather than refusing it: the path exists, and canonicalising it is
// what realpath is for.
func TestDeviceEdges_realpathAnswersForADevice(t *testing.T) {
	for _, test := range []struct{ operand, want string }{
		{operand: "/dev/null", want: "/dev/null"},
		{operand: "/dev", want: "/dev"},
		{operand: "/dev/../dev/zero", want: "/dev/zero"},
	} {
		stdout, stderr, status := runScriptForDevices(t, "realpath "+test.operand+"\n")
		if status != 0 {
			t.Fatalf("realpath %s: status %d, stderr %q", test.operand, status, stderr)
		}
		if strings.TrimSpace(stdout) != test.want {
			t.Fatalf("realpath %s = %q, want %q", test.operand, strings.TrimSpace(stdout), test.want)
		}
	}
}

// A device that does not exist is still refused, so realpath has not been made to accept anything
// under /dev.
func TestDeviceEdges_realpathRefusesAnUnknownDevice(t *testing.T) {
	_, stderr, status := runScriptForDevices(t, "realpath /dev/nosuchthing\n")
	if status == 0 {
		t.Fatalf("realpath /dev/nosuchthing succeeded, stderr %q", stderr)
	}
}

// And the refusals that must survive: a device is not a program.
func TestDeviceEdges_aDeviceIsNotExecutable(t *testing.T) {
	_, stderr, status := runScriptForDevices(t, "/dev/null\n")
	if status == 0 {
		t.Fatal("/dev/null ran as a command")
	}
	if !strings.Contains(stderr, "not executable") {
		t.Fatalf("running /dev/null said %q, want it named as not executable", strings.TrimSpace(stderr))
	}
}
