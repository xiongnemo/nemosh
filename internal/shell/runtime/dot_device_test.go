package runtime_test

import "testing"

// `.` reads a device to its end as it reads a file, as busybox-w32 and bash do: /dev/null is an
// empty script, and /dev/stdin is whatever the shell's input holds, a pipe or a heredoc. Each
// was "not a regular file", status 1.
func TestRuntime_dotReadsADevice(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{". /dev/null; echo \"after $?\"\n", "after 0\n"},
		{"echo 'echo from-stdin; x=1' | { . /dev/stdin; echo \"x=$x\"; }\n", "from-stdin\nx=1\n"},
		{". /dev/stdin <<EOF\necho heredoc\nEOF\n", "heredoc\n"},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
			t.Errorf("%d: %q: got %q/%d, want %q/0, as busybox-w32 and bash answer", index, test.script, stdout, status, test.want)
		}
	}
}
