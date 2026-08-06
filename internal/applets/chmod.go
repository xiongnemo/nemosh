package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
)

func newChmodApplet() Applet {
	return simpleApplet{name: "chmod", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		if len(args) < 2 {
			return ErrExitFalse
		}
		mode, err := parseChmodMode(args[0])
		if err != nil {
			return fmt.Errorf("%s: invalid mode", args[0])
		}
		view := ProcessViewFromContext(ctx)
		for _, path := range args[1:] {
			native, err := resolveHostPath(view, path)
			if err != nil {
				return err
			}
			if err := os.Chmod(native, mode); err != nil {
				return operandFailure(path, err)
			}
		}
		return nil
	}}
}

func parseChmodMode(raw string) (os.FileMode, error) {
	mode, err := strconv.ParseUint(raw, 8, 32)
	if err != nil || mode > 0o7777 {
		return 0, strconv.ErrSyntax
	}
	return os.FileMode(mode), nil
}
