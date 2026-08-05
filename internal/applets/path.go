package applets

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

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
	view := ProcessViewFromContext(ctx)
	_, typed := view.(pathProcessView)
	model, modelErr := cwdPathModel(view.WorkingDirectory())
	for _, arg := range args {
		converted, err := "", modelErr
		lexical, recognized, lexicalErr := model.ResolveWindowsSpelling(arg)
		if recognized {
			err = lexicalErr
			if err == nil && a.name == "winpath" {
				converted, err = pathmodel.WindowsPath(lexical)
			} else if err == nil {
				converted = string(lexical)
			}
		} else if typed {
			resolved, resolveErr := ResolveProcessPath(view, arg)
			if resolveErr != nil {
				err = resolveErr
			} else if a.name == "winpath" {
				converted, err = pathmodel.WindowsPath(resolved.Canonical)
				if err != nil && isTmpPath(resolved.Canonical) && resolved.Native != "" {
					converted = filepath.ToSlash(resolved.Native)
					err = nil
				} else if resolved.Device {
					err = pathmodel.ErrNoWindowsPath
				}
			} else {
				converted = string(resolved.Canonical)
				err = nil
			}
		} else if err == nil {
			converted, err = a.convert(model, arg)
		}
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

func isTmpPath(path pathmodel.Path) bool {
	return path == "/tmp" || strings.HasPrefix(string(path), "/tmp/")
}

func cwdPathModel(cwd string) (pathmodel.Model, error) {
	seed := pathmodel.New(pathmodel.DefaultConfig(), "/c")
	path, err := seed.Resolve(cwd)
	if err != nil {
		return pathmodel.Model{}, fmt.Errorf("resolve cwd %q: %w", cwd, err)
	}
	return pathmodel.New(pathmodel.DefaultConfig(), path), nil
}
