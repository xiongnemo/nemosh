package applets

import (
	"context"
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
		view := ProcessViewFromContext(ctx)
		paths, expression, err := parseFindArguments(args, view)
		if err != nil {
			return err
		}
		run := &findRun{stdout: stdout}
		for _, root := range paths {
			// A device path has no host root to walk, so it is walked from the
			// table instead. The callback is the same one: an entry is an
			// fs.DirEntry either way, which is what makes `find /dev -type c`
			// work without find knowing what a device is.
			handled, err := walkDeviceRoot(view, root, func(path string, entry fs.DirEntry) error {
				depth := 0
				if path != root {
					depth = 1
				}
				return expression.evaluate(findCandidate{display: path, entry: entry, depth: depth}, run)
			})
			if err != nil {
				return err
			}
			if handled {
				continue
			}
			hostRoot, err := resolveHostPath(view, root)
			if err != nil {
				return err
			}
			if err := walkFindPath(run, root, hostRoot, expression); err != nil {
				return err
			}
		}
		return run.err
	}}
}

func walkFindPath(run *findRun, displayRoot, hostRoot string, expression findExpression) error {
	return filepath.WalkDir(hostRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		display, depth, err := findDisplayPath(displayRoot, hostRoot, path)
		if err != nil {
			return err
		}
		// An unreadable entry is reported against the name the walk was asked
		// for, not the host path the walk happens to be standing on.
		if walkErr != nil {
			return operandFailure(display, walkErr)
		}
		candidate := findCandidate{display: display, host: path, entry: entry, depth: depth}
		if err := expression.evaluate(candidate, run); err != nil {
			return err
		}
		// Pruning rather than filtering: -maxdepth 1 must stop the walk from
		// reading a subdirectory, not read it and discard the entries.
		if entry != nil && entry.IsDir() && expression.prunes(candidate) {
			return fs.SkipDir
		}
		return nil
	})
}

// findDisplayPath builds what POSIX says find writes: the path operand as the
// user spelled it, a slash, and the rest of the path. The operand is not
// cleaned, which is why this concatenates instead of using filepath.Join --
// Join would turn `find .` into `a.txt` where every other find, busybox
// included, writes `./a.txt`, and a script comparing or stripping that prefix
// would silently see different text.
//
// The depth it returns is what -maxdepth and -mindepth count: zero for the
// operand itself, and one per path element below it.
func findDisplayPath(displayRoot, hostRoot, path string) (string, int, error) {
	relative, err := filepath.Rel(hostRoot, path)
	if err != nil {
		return "", 0, err
	}
	root := filepath.ToSlash(displayRoot)
	if relative == "." {
		return root, 0, nil
	}
	slashed := filepath.ToSlash(relative)
	depth := strings.Count(slashed, "/") + 1
	if strings.HasSuffix(root, "/") {
		return root + slashed, depth, nil
	}
	return root + "/" + slashed, depth, nil
}
