package main

import (
	"context"
	"strings"
	"testing"
)

// The word after a command that runs a command is a command name: `sudo gi<TAB>` is `sudo git`.
// It completed files, since sudo's operand was taken for a path like any other. The wrapper's
// options are stepped over, with the value an option takes -- `xargs -n 1 gi` -- and env's
// assignments; and an assignment before a command leaves the next word a command name, as bash
// has it. A command that takes files still completes files.
func TestLineEditor_completesTheCommandAWrapperRuns(t *testing.T) {
	for _, test := range []struct{ typed, want string }{
		{"sudo gi", "sudo git "},
		{"env FOO=1 BAR=2 gi", "env FOO=1 BAR=2 git "},
		{"env -u HOME gi", "env -u HOME git "},
		{"xargs -n 1 gi", "xargs -n 1 git "},
		{"time gi", "time git "},
		{"command -v gi", "command -v git "},
		{"sudo env FOO=1 gi", "sudo env FOO=1 git "},
		{"A=1 gi", "A=1 git "},
		{"ls | xargs gi", "ls | xargs git "},
		{"cat gi", "cat gi"},
	} {
		t.Run(test.typed, func(t *testing.T) {
			// Given
			editor := newLineEditor(strings.NewReader(test.typed+"\t\r"), newScreenModel(t, 120), t.TempDir())
			editor.width = func() int { return 120 }
			editor.commands = newShellCommands(settledIndex(t, seedPathDirectory(t, program("git"))))

			// When
			line, err := editor.readLine(context.Background(), "$ ")

			// Then
			if err != nil {
				t.Fatalf("readLine: %v", err)
			}
			if line != test.want {
				t.Fatalf("line = %q, want %q", line, test.want)
			}
		})
	}
}
