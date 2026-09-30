package applets

import (
	"io"
	"runtime"
)

// reset is busybox's (console-tools/reset.c): on a terminal it puts the display back as it
// was at the start and then does what `stty sane` does. busybox-w32's resets the attributes,
// leaves the alternate screen and clears the scrollback and the screen; busybox's elsewhere
// resets the terminal whole, the character set, the attributes and the cursor. It takes no
// options and reads no operands, as busybox's reads none, and does nothing when its output is
// not a terminal.
func newResetApplet() Applet {
	return simpleApplet{name: "reset", run: func(_ []string, _ io.Reader, stdout, _ io.Writer) error {
		if !stdoutIsTerminal(stdout) {
			return nil
		}
		sequence := "\033c\033(B\033[m\033[J\033[?25h"
		if runtime.GOOS == "windows" {
			sequence = "\033[m\033[?1049l\033[3J\033[H\033[2J"
		}
		if _, err := io.WriteString(stdout, sequence); err != nil {
			return err
		}
		if handle, ok := sttyTerminal(); ok {
			return makeTerminalSane(handle)
		}
		return nil
	}}
}
