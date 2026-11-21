package runtime

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// askedTerminal is a stream that records being asked which file it ends at, as `test -t`
// asks one; it names none.
type askedTerminal struct{ asked bool }

func (w *askedTerminal) Write(p []byte) (int, error) { return len(p), nil }

func (w *askedTerminal) TerminalFile() *os.File {
	w.asked = true
	return nil
}

// `[[ -t 1 ]]` asks the shell's standard output, as `[ -t 1 ]` does. It asked nothing -- `[[`
// handed `test`'s primaries no streams -- so on a real console it said no where `[` said yes,
// and bash-preexec, whose DEBUG hook returns at once when `[[ ! -t 1 ]]`, never ran a preexec:
// atuin recorded no history. Measured in a console the robot drove, 2026-11-21.
func TestDoubleBracket_terminalTestAsksTheShellsStreams(t *testing.T) {
	for _, script := range []string{"[ -t 1 ]\n", "[[ -t 1 ]]\n", "[[ ! -t 1 ]]\n", "[[ -t 2 ]]\n"} {
		stdout, stderr := &askedTerminal{}, &askedTerminal{}
		rt := New(applets.DefaultRegistry, Streams{Stdin: new(bytes.Buffer), Stdout: stdout, Stderr: stderr})
		rt.RunScript(context.Background(), script)
		if !stdout.asked && !stderr.asked {
			t.Errorf("%q asked neither stream whether it is a terminal", script)
		}
	}
}
