package runtime_test

import (
	"strings"
	"testing"
)

// A command name that cannot be a filename does not exist, and the shell should
// say so in the words it uses for everything else that does not exist.
//
// It used to leak the Win32 failure instead:
//
//	^M: stat executable "C:\Users\nemo\bin\\r": CreateFile ...: The filename,
//	directory name, or volume label syntax is incorrect.
//
// busybox answers `^M: not found`. Found by the parser fuzzer, which reached it
// through `0|<CR>` -- a carriage return is data there, so it becomes a command
// name.
func TestLookup_reportsNotFound_whenTheNameCannotBeAFilename(t *testing.T) {
	for _, test := range []struct {
		name    string
		command string
	}{
		{name: "carriage return", command: "\r"},
		{name: "a control character", command: "\x01"},
		{name: "a character Windows reserves", command: "a<b"},
		{name: "a pipe in the name", command: "a|b"},
		{name: "a quote in the name", command: "a\"b"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, _, stderr := runSetScript(t, "'"+strings.ReplaceAll(test.command, "'", "")+"'\n")

			// Then
			if status != 127 {
				t.Errorf("status = %d, stderr = %q, want 127", status, stderr)
			}
			if !strings.Contains(stderr, "not found") {
				t.Errorf("stderr = %q, want it to say not found", stderr)
			}
			for _, leaked := range []string{"CreateFile", "stat executable", "syntax is incorrect"} {
				if strings.Contains(stderr, leaked) {
					t.Errorf("stderr = %q leaks %q", stderr, leaked)
				}
			}
		})
	}
}
