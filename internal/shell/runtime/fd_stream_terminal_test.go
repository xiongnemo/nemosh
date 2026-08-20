package runtime

import (
	"bytes"
	"os"
	"testing"
)

// An applet has to be able to ask what its output ends at, and could not.
//
// Everything an applet writes to inside the shell is a descriptorWriter, never os.Stdout, so
// `stdout.(*os.File)` was false whatever the shell was attached to. Two features looked absent
// as a result: `ls` laid out no columns even on a terminal, and `ls --color=auto` coloured
// nothing. Neither was missing -- the question was being asked of the wrong object, and the
// answer was always no.
//
// This tests the plumbing rather than the terminal: whether a *terminal* is on the other end is
// x/term's business, and cannot be arranged in a test at all. What can be arranged is that the
// file is reachable when there is one and absent when there is not.
func TestDescriptorWriter_TerminalFile(t *testing.T) {
	t.Run("reaches the file the shell was given", func(t *testing.T) {
		// Given a shell writing to a real file, as an interactive one is
		table := newFDTable(Streams{Stdout: os.Stdout, Stderr: os.Stderr, Stdin: os.Stdin})
		writer, ok := table.streams().Stdout.(descriptorWriter)
		if !ok {
			t.Fatalf("Stdout is %T, want a descriptorWriter", table.streams().Stdout)
		}

		// When
		file := writer.TerminalFile()

		// Then
		if file != os.Stdout {
			t.Fatalf("TerminalFile() = %v, want os.Stdout", file)
		}
	})

	t.Run("is nil for a buffer", func(t *testing.T) {
		// Given the shape every test and every pipeline stage uses
		table := newFDTable(Streams{Stdout: &bytes.Buffer{}})
		writer := table.streams().Stdout.(descriptorWriter)

		// When
		file := writer.TerminalFile()

		// Then -- nil is the answer that matters: a redirected ls must behave as it does
		// into a pipe.
		if file != nil {
			t.Fatalf("TerminalFile() = %v, want nil", file)
		}
	})
}
