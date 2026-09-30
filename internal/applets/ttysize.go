package applets

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// ttysize is busybox's (miscutils/ttysize.c): the terminal's width and height, `80 24` when no
// standard stream is one. With operands it prints, for each, the width for one beginning with w
// and the height for one beginning with h, a blank before all but the first, and nothing for
// any other, as busybox reads them. It takes no options, so `-Z` is an operand like any other.
func newTtysizeApplet() Applet {
	return simpleApplet{name: "ttysize", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		width, height := 80, 24
		if handle, ok := sttyTerminal(); ok {
			if columns, rows, err := term.GetSize(int(handle.Fd())); err == nil {
				width, height = columns, rows
			}
		}
		if len(args) == 0 {
			_, err := fmt.Fprintf(stdout, "%d %d\n", width, height)
			return err
		}
		var line strings.Builder
		separator := ""
		for _, arg := range args {
			switch {
			case strings.HasPrefix(arg, "w"):
				line.WriteString(separator + strconv.Itoa(width))
			case strings.HasPrefix(arg, "h"):
				line.WriteString(separator + strconv.Itoa(height))
			}
			separator = " "
		}
		_, err := fmt.Fprintln(stdout, line.String())
		return err
	}}
}
