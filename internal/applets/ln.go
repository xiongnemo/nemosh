package applets

import (
	"fmt"
	"io"
	"os"
)

func newLnApplet() Applet {
	return simpleApplet{name: "ln", run: func(args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		symbolic := false
		paths := args
		if len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
			if args[0] != "-s" {
				return fmt.Errorf("unsupported ln option: %s", args[0])
			}
			symbolic = true
			paths = args[1:]
		}
		if len(paths) != 2 {
			return ErrExitFalse
		}
		if symbolic {
			return os.Symlink(paths[0], paths[1])
		}
		return os.Link(paths[0], paths[1])
	}}
}
