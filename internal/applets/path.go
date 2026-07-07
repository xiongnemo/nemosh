package applets

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

func newWinpathApplet() Applet {
	return pathApplet{name: "winpath", convert: func(model pathmodel.Model, arg string) (string, error) {
		path, err := model.Resolve(arg)
		if err != nil {
			return "", err
		}
		return pathmodel.WindowsPath(path)
	}}
}

func newPosixpathApplet() Applet {
	return pathApplet{name: "posixpath", convert: func(model pathmodel.Model, arg string) (string, error) {
		path, err := model.Resolve(arg)
		if err != nil {
			return "", err
		}
		return string(path), nil
	}}
}

type pathApplet struct {
	name    string
	convert func(model pathmodel.Model, arg string) (string, error)
}

func (a pathApplet) Name() string {
	return a.name
}

func (a pathApplet) Run(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if len(args) == 0 {
		return ErrExitFalse
	}
	model, err := cwdPathModel()
	if err != nil {
		return err
	}
	for _, arg := range args {
		converted, err := a.convert(model, arg)
		if err != nil {
			if _, writeErr := fmt.Fprintln(stderr, err); writeErr != nil {
				return writeErr
			}
			return ErrExitFalse
		}
		if _, err := fmt.Fprintln(stdout, converted); err != nil {
			return err
		}
	}
	return nil
}

func cwdPathModel() (pathmodel.Model, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return pathmodel.Model{}, fmt.Errorf("get cwd: %w", err)
	}
	seed := pathmodel.New(pathmodel.DefaultConfig(), "/c")
	path, err := seed.Resolve(cwd)
	if err != nil {
		return pathmodel.Model{}, fmt.Errorf("resolve cwd %q: %w", cwd, err)
	}
	return pathmodel.New(pathmodel.DefaultConfig(), path), nil
}
