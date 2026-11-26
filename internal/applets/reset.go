package applets

import "io"

// reset is busybox-w32's (console-tools/reset.c): on a terminal it resets the attributes,
// leaves the alternate screen, clears the scrollback and the screen, and then does what `stty
// sane` does. It takes no options and reads no operands, as busybox's reads none, and does
// nothing when its output is not a terminal.
func newResetApplet() Applet {
	return simpleApplet{name: "reset", run: func(_ []string, _ io.Reader, stdout, _ io.Writer) error {
		if !stdoutIsTerminal(stdout) {
			return nil
		}
		if _, err := io.WriteString(stdout, "\033[m\033[?1049l\033[3J\033[H\033[2J"); err != nil {
			return err
		}
		if handle, ok := sttyTerminal(); ok {
			defer noteSttySet(handle)
			return makeTerminalSane(handle)
		}
		return nil
	}}
}
