package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func newRealpathApplet() Applet {
	return simpleApplet{name: "realpath", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletLongOptions(ctx, args, realpathLongOptions, "emsqz", realpathRelativeTo+realpathRelativeBase)
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return ErrExitFalse
		}
		view := ProcessViewFromContext(ctx)
		run, err := newRealpathRun(view, options)
		if err != nil {
			return err
		}
		failed := false
		for _, arg := range operands {
			resolved, err := ResolveProcessPath(view, currentIfEmpty(arg))
			if err == nil && resolved.Device {
				// A device is a real path with a canonical spelling, which is exactly
				// what realpath is for: `/dev/../dev/zero` answers `/dev/zero`. There
				// is no host path to hand to realpath() below, and none is needed --
				// the canonical form *is* the answer.
				if info, statErr := statDeviceOperand(view, arg); statErr == nil && info != nil {
					if _, writeErr := fmt.Fprint(stdout, resolved.Canonical, run.end); writeErr != nil {
						return writeErr
					}
					continue
				}
				// Under /dev and not a device this shell has. No such path, which is
				// what the message should say rather than blaming the host for a name
				// the host was never asked about.
				err = errRealpathNoDevice
			}
			native := resolved.Native
			if err == nil {
				native, err = run.canonical(native)
			}
			if err != nil {
				failed = true
				if writeErr := run.report(stderr, arg, err); writeErr != nil {
					return writeErr
				}
				continue
			}
			shown := run.relative(native, canonicalizeGeneratedPath(view, resolved.Canonical, native))
			if _, err := fmt.Fprint(stdout, shown, run.end); err != nil {
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
	return realpathAbs(path, false)
}

// realpathAbs resolves path, where missingAllowed says whether a path that is not there is
// an answer or a failure.
//
// The distinction is the whole of what `realpath` has to get right about absence, and both
// halves were measured against busybox-w32, which is the reference this shell follows:
//
//	realpath nosuch    -> realpath: nosuch: No such file or directory, status 1
//	realpath dangling  -> the target it points at, status 0
//
// So an operand that does not exist fails, and a *symlink target* that does not exist does
// not -- a dangling link still names a path, and printing it is what makes `realpath` usable
// for "where would this go". GNU and uutils agree on both. Before this, the first form
// printed a path and exited 0, which is `realpath -m` behaviour that neither reference has
// and that no operand asked for.
func realpathAbs(path string, missingAllowed bool) (string, error) {
	resolved, err := FollowLinks(path)
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
		// Following a link is what licenses the absence below.
		return realpathAbs(target, true)
	}
	if !missingAllowed {
		return "", err
	}

	parent, leaf := splitFinalPathElement(path)
	resolvedParent, parentErr := FollowLinks(parent)
	if parentErr != nil {
		return "", parentErr
	}
	return filepath.ToSlash(filepath.Clean(filepath.Join(resolvedParent, leaf))), nil
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
