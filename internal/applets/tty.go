package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// tty, coreutils': whether standard input is a terminal, and its name. From a file or a pipe it is
// `not a tty` and status 1, as coreutils' is, and -s says the same by its status alone, which is
// what a script asks. The name a terminal has is refused for now, status 2: on Windows a console
// has no device path nemosh can open -- /dev/tty is not one of its devices yet, and that is a
// design of its own (docs/design/windows-path-model.md) -- and a name that opens nothing is worse
// than none. busybox-w32 has no tty.

func newTtyApplet() Applet {
	return simpleApplet{name: "tty", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletLongOptions(ctx, args, "silent=s quiet=s", "s", "")
		switch {
		case err != nil:
			return err
		case len(operands) > 0:
			return ExitStatusMessage(2, fmt.Errorf("extra operand '%s'", operands[0]))
		case !inputIsTerminal(ctx, stdin):
			if !options.has('s') {
				fmt.Fprintln(stdout, "not a tty")
			}
			return ExitStatus(1)
		case options.has('s'):
			return nil
		}
		return ExitStatusMessage(2, errors.New("this terminal has no device path here: /dev/tty is not one of nemosh's devices yet (docs/design/windows-path-model.md); tty -s says whether there is a terminal"))
	}}
}
