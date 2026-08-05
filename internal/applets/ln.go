package applets

import (
	"context"
	"fmt"
	"io"
	"os"
)

func newLnApplet() Applet {
	return simpleApplet{name: "ln", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
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
		view := ProcessViewFromContext(ctx)
		if symbolic {
			linkName, err := resolveHostPath(view, paths[1])
			if err != nil {
				return err
			}
			return os.Symlink(paths[0], linkName)
		}
		source, err := resolveHostPath(view, paths[0])
		if err != nil {
			return err
		}
		linkName, err := resolveHostPath(view, paths[1])
		if err != nil {
			return err
		}
		return os.Link(source, linkName)
	}}
}
