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
	return filepath.WalkDir(hostRoot, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, relErr := filepath.Rel(hostRoot, path)
		if relErr != nil {
			return relErr
		}
		display := displayRoot
		if relative != "." {
			display = filepath.Join(displayRoot, relative)
		}
		_, printErr := fmt.Fprintln(stdout, filepath.ToSlash(display))
		return printErr
	})
}
