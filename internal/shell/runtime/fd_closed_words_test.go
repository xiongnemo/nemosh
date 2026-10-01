package runtime_test

import "testing"

// A write or a read through a descriptor `>&-` or `<&-` closed says so in busybox's applets'
// words, `cat: write error: Bad file descriptor` and `read error` for a read, where it said
// "file descriptor is closed"; the status is 1, as it was.
func TestClosedDescriptor_isAWriteOrAReadError(t *testing.T) {
	for _, test := range []struct{ script, stderr string }{
		{script: "echo hi >&-; echo \"st=$?\" >&2\n", stderr: "echo: write error: Bad file descriptor\nst=1\n"},
		{script: "printf x >&-; echo \"st=$?\" >&2\n", stderr: "printf: write error: Bad file descriptor\nst=1\n"},
		{script: "cat <&-; echo \"st=$?\" >&2\n", stderr: "cat: read error: Bad file descriptor\nst=1\n"},
		{script: "read x <&-; echo \"st=$?\" >&2\n", stderr: "read: read error: Bad file descriptor\nst=1\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if _, _, stderr := runSetScript(t, test.script); stderr != test.stderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.stderr)
			}
		})
	}
}
