package applets

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func newRealpathApplet() Applet {
	return simpleApplet{name: "realpath", run: func(args []string, _ io.Reader, stdout, stderr io.Writer) error {
		if len(args) == 0 {
			return ErrExitFalse
		}

		failed := false
		for _, arg := range args {
			resolved, err := realpath(arg)
			if err != nil {
				failed = true
				if _, writeErr := fmt.Fprintf(stderr, "realpath: %s: %v\n", filepath.ToSlash(arg), err); writeErr != nil {
					return writeErr
				}
				continue
			}
			if _, err := fmt.Fprintln(stdout, resolved); err != nil {
				return err
			}
		}
		if failed {
			return ErrExitFalse
		}
		return nil
	}}
}

func realpath(path string) (string, error) {
	abs, err := absoluteRealpathOperand(path)
	if err != nil {
		return "", fmt.Errorf("absolute path: %w", err)
	}
	return realpathAbs(abs)
}

func realpathAbs(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.ToSlash(filepath.Clean(resolved)), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}

	target, readlinkErr := os.Readlink(path)
	if readlinkErr == nil {
		if !filepath.IsAbs(target) {
			target = appendPath(filepath.Dir(path), target)
		}
		return realpath(target)
	}

	parent, leaf := splitFinalPathElement(path)
	resolvedParent, parentErr := filepath.EvalSymlinks(parent)
	if parentErr != nil {
		return "", parentErr
	}
	return filepath.ToSlash(filepath.Clean(filepath.Join(resolvedParent, leaf))), nil
}

func absoluteRealpathOperand(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return appendPath(cwd, path), nil
}

func splitFinalPathElement(path string) (string, string) {
	trimmed := strings.TrimRightFunc(path, isPathSeparator)
	if trimmed == "" {
		trimmed = path
	}
	parent, leaf := filepath.Split(trimmed)
	if parent == "" {
		parent = "."
	}
	return parent, leaf
}

func appendPath(parent string, child string) string {
	if parent == "" {
		return child
	}
	if strings.HasSuffix(parent, "/") || strings.HasSuffix(parent, `\`) {
		return parent + child
	}
	return parent + string(os.PathSeparator) + child
}

func isPathSeparator(char rune) bool {
	return char == '/' || char == '\\'
}
