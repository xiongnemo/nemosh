package applets

import (
	"context"
	"fmt"
	"io"
	"os"
)

func newReadlinkApplet() Applet {
	return simpleApplet{name: "readlink", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		newline := true
		paths := args
		if len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
			if args[0] != "-n" {
				return fmt.Errorf("unsupported readlink option: %s", args[0])
			}
			newline = false
			paths = args[1:]
		}
		if len(paths) != 1 {
			return ErrExitFalse
		}
		linkName, err := resolveHostPath(ProcessViewFromContext(ctx), paths[0])
		if err != nil {
			return err
		}
		target, err := os.Readlink(linkName)
		if err != nil {
			return ErrExitFalse
		}
		if newline {
			_, err = fmt.Fprintln(stdout, target)
			return err
		}
		_, err = fmt.Fprint(stdout, target)
		return err
	}}
}
