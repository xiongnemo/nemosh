package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type touchApplet struct{}

func newTouchApplet() Applet {
	return touchApplet{}
}

func (touchApplet) Name() string { return "touch" }
func (touchApplet) Run(ctx context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
	// `touch -z` used to create a file called -z.
	_, operands, err := parseAppletOptions(args, "c", "")
	if err != nil {
		return err
	}
	if len(operands) == 0 {
		return missingOperand()
	}
	view := ProcessViewFromContext(ctx)
	for _, path := range operands {
		native, err := resolveHostPath(view, path)
		if err != nil {
			return err
		}
		file, err := os.OpenFile(native, os.O_CREATE|os.O_WRONLY, 0o666)
		if err != nil {
			return operandFailure(path, err)
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

func newRmApplet() Applet {
	return simpleApplet{name: "rm", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(args, "fr", "")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			// -f makes a missing operand acceptable, which is the whole point
			// of `rm -f` in a cleanup script.
			if options.has('f') {
				return nil
			}
			return missingOperand()
		}
		view := ProcessViewFromContext(ctx)
		removed := true
		for _, path := range operands {
			native, err := resolveHostPath(view, path)
			if err != nil {
				fmt.Fprintf(stderr, "rm: %v\n", err)
				removed = false
				continue
			}
			if !removeOperand(native, path, options.has('r'), options.has('f'), stderr) {
				removed = false
			}
		}
		if !removed {
			// Every failure has been reported already, so this carries the
			// status and nothing else. Stopping at the first one instead left a
			// cleanup half done and named none of what survived.
			return ExitStatus(1)
		}
		return nil
	}}
}

func removeOperand(native, display string, recursive, force bool, stderr io.Writer) bool {
	if recursive {
		return removeTree(native, display, force, stderr)
	}
	info, err := os.Lstat(native)
	if err != nil {
		// -f is silent about what was not there to begin with, which is what
		// makes `rm -f build.out` usable in a cleanup script.
		if force && errors.Is(err, fs.ErrNotExist) {
			return true
		}
		reportRemoveFailure(stderr, display, err)
		return false
	}
	// Lstat rather than Stat, so a symlink to a directory is unlinked instead
	// of being refused -- the link is not a directory, whatever it points at.
	if info.IsDir() {
		return reportIsADirectory(stderr, display)
	}
	return removeOne(native, display, stderr)
}

// mkdir takes -p and -m, the two options busybox's getopt32long string carries
// besides -v (coreutils/mkdir.c:63). Without option parsing the flags were
// taken as operands, so `mkdir -p a/b/c` created a directory literally named
// -p and then failed to create a/b/c because its parents were missing.
func newMkdirApplet() Applet {
	return simpleApplet{name: "mkdir", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		options, operands, err := parseAppletOptions(args, "pv", "m")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		mode, err := mkdirMode(options)
		if err != nil {
			return err
		}
		view := ProcessViewFromContext(ctx)
		for _, path := range operands {
			native, err := resolveHostPath(view, path)
			if err != nil {
				return err
			}
			if err := makeDirectory(native, mode, options.has('p')); err != nil {
				return cannotCreateDirectory(path, err)
			}
		}
		return nil
	}}
}

func mkdirMode(options appletOptions) (os.FileMode, error) {
	if !options.has('m') {
		return 0o777, nil
	}
	parsed, err := parseChmodMode(options.value('m'))
	if err != nil {
		return 0, fmt.Errorf("invalid mode '%s'", options.value('m'))
	}
	return parsed, nil
}

// -p makes every missing parent and accepts a target that is already a
// directory; without it only the last component is created and an existing one
// is an error.
func makeDirectory(native string, mode os.FileMode, parents bool) error {
	if !parents {
		return os.Mkdir(native, mode)
	}
	if info, err := os.Stat(native); err == nil && info.IsDir() {
		return nil
	}
	return os.MkdirAll(native, mode)
}

// rmdir removes directories and nothing else. It used to call os.Remove, which
// also removes files, so `rmdir notes.txt` deleted the file and reported
// success -- silent data loss against rmdir(3), which fails with ENOTDIR
// (coreutils/rmdir.c:73). The stat is a moment before the removal rather than
// part of it, which is a race Go's portable API leaves no way to close; what it
// buys is that the ordinary case of naming a file by mistake cannot destroy it.
func newRmdirApplet() Applet {
	return simpleApplet{name: "rmdir", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		options, operands, err := parseAppletOptions(args, "pv", "")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		view := ProcessViewFromContext(ctx)
		for _, path := range operands {
			if err := removeDirectoryTree(view, path, options.has('p')); err != nil {
				return err
			}
		}
		return nil
	}}
}

// -p walks up removing each parent in turn, stopping at the first one that will
// not go, exactly as busybox does with dirname in its loop.
func removeDirectoryTree(view ProcessView, path string, parents bool) error {
	for {
		native, err := resolveHostPath(view, path)
		if err != nil {
			return err
		}
		if err := removeEmptyDirectory(native); err != nil {
			return quotedFailure(path, err)
		}
		parent := filepath.Dir(path)
		if !parents || parent == path || parent == "." || parent == string(filepath.Separator) {
			return nil
		}
		path = parent
	}
}

func removeEmptyDirectory(native string) error {
	info, err := os.Lstat(native)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errNotADirectory
	}
	return os.Remove(native)
}
