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
	model, modelErr := cwdPathModel(view.WorkingDirectory())
	for _, arg := range args {
		converted, err := "", modelErr
		if err == nil {
			converted, err = convertedPath(view, model, arg, a.name == "winpath", a.convert)
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

// convertedPath is arg as winpath or, with windows false, posixpath answers it: absolute, in the
// shell's model, the Windows spelling with forward slashes or the Unix one. fallback converts
// where neither the Windows spelling nor a shell's view can.
func convertedPath(view ProcessView, model pathmodel.Model, arg string, windows bool, fallback func(pathmodel.Model, string) (string, error)) (string, error) {
	if lexical, recognized, err := model.ResolveWindowsSpelling(arg); recognized {
		if err != nil || !windows {
			return string(lexical), err
		}
		return pathmodel.WindowsPath(lexical)
	}
	if _, typed := view.(pathProcessView); !typed {
		return fallback(model, arg)
	}
	resolved, err := ResolveProcessPath(view, arg)
	switch {
	case err != nil:
		return "", err
	case !windows:
		return string(resolved.Canonical), nil
	}
	converted, err := pathmodel.WindowsPath(resolved.Canonical)
	if err != nil && isTmpPath(resolved.Canonical) && resolved.Native != "" {
		return filepath.ToSlash(resolved.Native), nil
	}
	if resolved.Device {
		return "", pathmodel.ErrNoWindowsPath
	}
	return converted, err
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
