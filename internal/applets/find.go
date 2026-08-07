package applets

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

func newFindApplet() Applet {
	return simpleApplet{name: "find", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		// The whole expression is validated before the first directory is read.
		// Walking first and reporting an unusable operand afterwards, which is
		// what this did, means the caller has already been handed every path in
		// the tree -- `find . -name '*.tmp' | xargs rm` received all of them.
		paths, expression, err := parseFindArguments(args)
		if err != nil {
			return err
		}
		view := ProcessViewFromContext(ctx)
		for _, root := range paths {
			hostRoot, err := resolveHostPath(view, root)
			if err != nil {
				return err
			}
			if err := walkFindPath(stdout, root, hostRoot, expression); err != nil {
				return err
			}
		}
		return nil
	}}
}

func walkFindPath(stdout io.Writer, displayRoot, hostRoot string, expression findExpression) error {
	return filepath.WalkDir(hostRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		display, err := findDisplayPath(displayRoot, hostRoot, path)
		if err != nil {
			return err
		}
		// An unreadable entry is reported against the name the walk was asked
		// for, not the host path the walk happens to be standing on.
		if walkErr != nil {
			return operandFailure(display, walkErr)
		}
		if !expression.matches(display, entry) {
			return nil
		}
		_, printErr := fmt.Fprintln(stdout, display)
		return printErr
	})
}

// findDisplayPath builds what POSIX says find writes: the path operand as the
// user spelled it, a slash, and the rest of the path. The operand is not
// cleaned, which is why this concatenates instead of using filepath.Join --
// Join would turn `find .` into `a.txt` where every other find, busybox
// included, writes `./a.txt`, and a script comparing or stripping that prefix
// would silently see different text.
func findDisplayPath(displayRoot, hostRoot, path string) (string, error) {
	relative, err := filepath.Rel(hostRoot, path)
	if err != nil {
		return "", err
	}
	root := filepath.ToSlash(displayRoot)
	if relative == "." {
		return root, nil
	}
	if strings.HasSuffix(root, "/") {
		return root + filepath.ToSlash(relative), nil
	}
	return root + "/" + filepath.ToSlash(relative), nil
}
