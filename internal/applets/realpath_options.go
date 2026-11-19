package applets

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// realpath's GNU options, which busybox's realpath has not got, so every word was a path:
// `realpath --relative-to=. x` looked for a file called --relative-to=.
//
//   - -e: every component must be there, a symbolic link's target too; -m: none need be; between
//     them, busybox's rule, a dangling link's target is an answer and a missing operand is not.
//   - -s and --strip: the path made absolute and cleaned, no link followed.
//   - --relative-to=DIR prints the path from DIR, and --relative-base=DIR prints it from DIR
//     only when it is under DIR, and absolute otherwise; given both, the path is printed from
//     --relative-to only when both are under the base, as GNU's has it.
//   - -q says nothing of what it cannot resolve; -z ends each with a NUL.

const (
	realpathRelativeTo   = "\x01"
	realpathRelativeBase = "\x02"
	realpathLongOptions  = "relative-to=" + realpathRelativeTo + " relative-base=" + realpathRelativeBase +
		" canonicalize-existing=e canonicalize-missing=m no-symlinks=s strip=s quiet=q zero=z"
)

type realpathRun struct {
	view                                 ProcessView
	existing, missing, noSymlinks, quiet bool
	end                                  string
	// relativeTo and base are --relative-to's and --relative-base's directories, resolved; empty
	// when not given.
	relativeTo, base string
}

func newRealpathRun(view ProcessView, options appletOptions) (*realpathRun, error) {
	run := &realpathRun{view: view, existing: options.last("em") == 'e', missing: options.last("em") == 'm',
		noSymlinks: options.has('s'), quiet: options.has('q'), end: "\n"}
	if options.has('z') {
		run.end = "\x00"
	}
	for _, option := range []struct {
		letter byte
		into   *string
	}{{realpathRelativeTo[0], &run.relativeTo}, {realpathRelativeBase[0], &run.base}} {
		if !options.has(option.letter) {
			continue
		}
		resolved, err := ResolveProcessPath(view, currentIfEmpty(options.value(option.letter)))
		if err == nil {
			*option.into, err = run.canonical(resolved.Native)
		}
		if err != nil {
			return nil, operandFailure(filepath.ToSlash(options.value(option.letter)), err)
		}
	}
	return run, nil
}

// canonical is native resolved as the options say.
func (r *realpathRun) canonical(native string) (string, error) {
	switch {
	case r.noSymlinks:
		return filepath.ToSlash(filepath.Clean(native)), nil
	case r.missing:
		return realpathMissing(native)
	case r.existing:
		resolved, err := FollowLinks(native)
		return filepath.ToSlash(filepath.Clean(resolved)), err
	}
	return realpath(native)
}

// realpathMissing is -m: the deepest part of path that is there resolved, and the rest after it.
func realpathMissing(path string) (string, error) {
	resolved, err := FollowLinks(path)
	if err == nil {
		return filepath.ToSlash(filepath.Clean(resolved)), nil
	}
	parent, leaf := splitFinalPathElement(path)
	if leaf == "" || filepath.Clean(parent) == filepath.Clean(path) {
		return "", err
	}
	head, err := realpathMissing(parent)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(head, "/") + "/" + leaf, nil
}

// relative is the path as --relative-to and --relative-base print it, or shown as it was.
func (r *realpathRun) relative(path, shown string) string {
	from := r.relativeTo
	if r.base != "" {
		if !realpathUnder(r.base, path) || from != "" && !realpathUnder(r.base, from) {
			return shown
		}
		if from == "" {
			from = r.base
		}
	}
	if from == "" {
		return shown
	}
	relative, err := filepath.Rel(filepath.FromSlash(from), filepath.FromSlash(path))
	if err != nil {
		return shown
	}
	return filepath.ToSlash(relative)
}

// realpathUnder is whether path is base or below it.
func realpathUnder(base, path string) bool {
	relative, err := filepath.Rel(filepath.FromSlash(base), filepath.FromSlash(path))
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// report says why an operand could not be resolved, unless -q.
func (r *realpathRun) report(stderr io.Writer, arg string, err error) error {
	if r.quiet {
		return nil
	}
	// operandFailure rather than the error itself: a *fs.PathError prints the host path it failed
	// on, which arrives with native separators mixed into an operand the caller spelled with
	// slashes -- `GetFileAttributesEx C:/Users\nemo\...` was the actual output. See diagnostic.go,
	// which exists for this.
	_, writeErr := fmt.Fprintf(stderr, "realpath: %v\n", operandFailure(filepath.ToSlash(arg), err))
	return writeErr
}

// errRealpathNoDevice is a path under /dev that names no device this shell has.
var errRealpathNoDevice = errors.New("No such file or directory")
