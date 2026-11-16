package runtime

import (
	"fmt"
	"io"
)

// titleBuiltin is busybox-w32's title (shell/ash.c:16063-16075): with an operand the console's
// title becomes it -- the first operand alone, whatever it looks like, for busybox reads no
// options -- and with none the title is printed. busybox answers 0 and says nothing when the
// console cannot be asked; this says why and answers 1, as a capability that is absent must.
func (r Runtime) titleBuiltin(args []string) int {
	return runTitle(args, hostConsoleTitle{}, r.streams.Stdout, r.streams.Stderr, r.diagnosticPrefix())
}

// consoleTitle is the console whose title is asked, which a test makes up.
type consoleTitle interface {
	title() (string, error)
	setTitle(text string) error
}

func runTitle(args []string, console consoleTitle, stdout, stderr io.Writer, prefix string) int {
	if len(args) > 0 {
		if err := console.setTitle(args[0]); err != nil {
			fmt.Fprintf(stderr, "%stitle: %v\n", prefix, err)
			return 1
		}
		return 0
	}
	text, err := console.title()
	if err != nil {
		fmt.Fprintf(stderr, "%stitle: %v\n", prefix, err)
		return 1
	}
	fmt.Fprintln(stdout, text)
	return 0
}
