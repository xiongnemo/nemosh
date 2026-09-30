package runtime_test

import (
	"strings"
	"testing"
)

// An applet opens a device it is given the name of, as a redirection does: tee, dd, sort -o,
// uniq's OUTPUT and sed's w and r. /dev/stdout, /dev/stderr and /dev/fd/N are the shell's own
// descriptors, lent for the applet's run, and /dev/null and /dev/zero are the shell's devices
// where the system has none. Each was "not a host path": `tee /dev/stderr`, the way a pipeline
// shows what passes through it, and `dd if=/dev/zero of=img` failed on every system, the
// descriptor names there too. Each answer is busybox-w32's, measured, but for `sed 'w
// /dev/stdout'`, whose lines busybox-w32 prints in the order its buffers are flushed.
func TestRuntime_appletsOpenDevicesByName(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"echo hi | tee /dev/null\n", "hi\n"},
		{"echo hi | tee /dev/stdout\n", "hi\nhi\n"},
		{"echo hi | tee /dev/fd/1\n", "hi\nhi\n"},
		{"{ echo hi | tee /dev/stderr >/dev/null; } 2>&1\n", "hi\n"},
		{"echo hi | tee -a /dev/null f; cat f\n", "hi\nhi\n"},
		{"echo hi | env tee /dev/stdout\n", "hi\nhi\n"},
		{"dd if=/dev/zero of=/dev/null bs=1k count=4 2>&1\n", "4+0 records in\n4+0 records out\n"},
		{"dd if=/dev/zero bs=1k count=2 2>/dev/null | wc -c\n", "2048\n"},
		{"echo hi | dd of=/dev/stdout 2>/dev/null\n", "hi\n"},
		{"echo abc | dd of=/dev/null seek=2 2>&1\n", "0+1 records in\n0+1 records out\n"},
		{"printf 'b\\na\\n' | sort -o /dev/stdout\n", "a\nb\n"},
		{"printf 'a\\na\\nb\\n' | uniq - /dev/stdout\n", "a\nb\n"},
		{"printf 'a\\nb\\n' | sed 'w /dev/stdout'\n", "a\na\nb\nb\n"},
		{"{ printf 'a\\nb\\n' | sed -n 'w /dev/stderr'; } 2>&1\n", "a\nb\n"},
		{"echo hi | sed 'r /dev/stdin'\n", "hi\n"},
		// The descriptor is the shell's, written where it stands, as busybox-w32's is.
		{"{ echo 12345; printf 'b\\na\\n' | sort -o /dev/stdout; } > f; cat f\n", "12345\na\nb\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q, status %d; want %q", stdout, status, test.want)
			}
		})
	}
}

// dd's seek= on a descriptor that cannot seek fails as busybox's lseek does there, and a
// FILE that is no device the shell has is not there.
func TestRuntime_appletDeviceFailures(t *testing.T) {
	for _, test := range []struct{ script, prefix string }{
		{"echo abc | dd of=/dev/stdout seek=1 2>&1\n", "dd: /dev/stdout: "},
		{"echo hi | tee /dev/fd/9 2>&1 >/dev/null\n", "tee: /dev/fd/9: "},
	} {
		t.Run(test.script, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if stdout, status := runScriptCapturing(test.script); !strings.HasPrefix(stdout, test.prefix) || status != 1 {
				t.Errorf("got %q, status %d; want %q..., status 1", stdout, status, test.prefix)
			}
		})
	}
}
