package applets

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
)

func newFindApplet() Applet {
	return simpleApplet{name: "find", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		paths := args
		if len(paths) == 0 {
			paths = []string{"."}
		}
		for _, root := range paths {
			if err := walkFindPath(stdout, root); err != nil {
				return err
			}
		}
		return nil
	}}
}

func walkFindPath(stdout io.Writer, root string) error {
	return filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		_, printErr := fmt.Fprintln(stdout, filepath.ToSlash(path))
		return printErr
	})
}
