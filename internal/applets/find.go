package applets

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
)

func newFindApplet() Applet {
	return simpleApplet{name: "find", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		paths := args
		if len(paths) == 0 {
			paths = []string{"."}
		}
		view := ProcessViewFromContext(ctx)
		for _, root := range paths {
			hostRoot, err := resolveHostPath(view, root)
			if err != nil {
				return err
			}
			if err := walkFindPath(stdout, root, hostRoot); err != nil {
				return err
			}
		}
		return nil
	}}
}

func walkFindPath(stdout io.Writer, displayRoot, hostRoot string) error {
	return filepath.WalkDir(hostRoot, func(path string, _ fs.DirEntry, walkErr error) error {
		display, err := findDisplayPath(displayRoot, hostRoot, path)
		if err != nil {
			return err
		}
		// An unreadable entry is reported against the name the walk was asked
		// for, not the host path the walk happens to be standing on.
		if walkErr != nil {
			return operandFailure(display, walkErr)
		}
		_, printErr := fmt.Fprintln(stdout, display)
		return printErr
	})
}

func findDisplayPath(displayRoot, hostRoot, path string) (string, error) {
	relative, err := filepath.Rel(hostRoot, path)
	if err != nil {
		return "", err
	}
	if relative == "." {
		return filepath.ToSlash(displayRoot), nil
	}
	return filepath.ToSlash(filepath.Join(displayRoot, relative)), nil
}
