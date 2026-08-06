package applets

import (
	"context"
	"io"
	"os"
)

type touchApplet struct{}

func newTouchApplet() Applet {
	return touchApplet{}
}

func (touchApplet) Name() string { return "touch" }
func (touchApplet) Run(ctx context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
	if len(args) == 0 {
		return ErrExitFalse
	}
	view := ProcessViewFromContext(ctx)
	for _, path := range args {
		native, err := resolveHostPath(view, path)
		if err != nil {
			return err
		}
		file, err := os.OpenFile(native, os.O_CREATE|os.O_WRONLY, 0o666)
		if err != nil {
			return operandFailure(path, err)
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

func newRmApplet() Applet {
	return simpleApplet{name: "rm", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		if len(args) == 0 {
			return ErrExitFalse
		}
		view := ProcessViewFromContext(ctx)
		for _, path := range args {
			native, err := resolveHostPath(view, path)
			if err != nil {
				return err
			}
			if err := os.Remove(native); err != nil {
				return cannotRemove(path, err)
			}
		}
		return nil
	}}
}

func newMkdirApplet() Applet {
	return simpleApplet{name: "mkdir", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		if len(args) == 0 {
			return ErrExitFalse
		}
		view := ProcessViewFromContext(ctx)
		for _, path := range args {
			native, err := resolveHostPath(view, path)
			if err != nil {
				return err
			}
			if err := os.Mkdir(native, 0o777); err != nil {
				return cannotCreateDirectory(path, err)
			}
		}
		return nil
	}}
}

func newRmdirApplet() Applet {
	return simpleApplet{name: "rmdir", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		if len(args) == 0 {
			return ErrExitFalse
		}
		view := ProcessViewFromContext(ctx)
		for _, path := range args {
			native, err := resolveHostPath(view, path)
			if err != nil {
				return err
			}
			if err := os.Remove(native); err != nil {
				return quotedFailure(path, err)
			}
		}
		return nil
	}}
}
