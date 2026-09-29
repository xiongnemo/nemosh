package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type touchApplet struct{}

func newTouchApplet() Applet {
	return touchApplet{}
}

func (touchApplet) Name() string { return "touch" }

// Run sets each file's times to now, creating it unless -c says not to, as busybox's touch does.
// It goes on past an operand it cannot touch, names it on stderr, and exits 1 at the end. It
// only ever created files: an existing one kept its old time, -c was read and ignored, and the
// first failure abandoned the operands after it.
func (touchApplet) Run(ctx context.Context, args []string, _ io.Reader, _ io.Writer, stderr io.Writer) error {
	// `touch -z` used to create a file called -z.
	options, operands, err := parseAppletOptions(ctx, args, "c", "")
	if err != nil {
		return err
	}
	if len(operands) == 0 {
		return missingOperand()
	}
	view := ProcessViewFromContext(ctx)
	now, touched := time.Now(), true
	for _, path := range operands {
		// A path the shell's view refuses, a disabled /cygdrive, is returned as it is.
		native, err := resolveHostPath(view, path)
		if err != nil {
			return err
		}
		if err := touchFile(native, now, options.has('c')); err != nil {
			fmt.Fprintf(stderr, "touch: %v\n", operandFailure(path, err))
			touched = false
		}
	}
	if !touched {
		return ExitStatus(1)
	}
	return nil
}

// touchFile sets an existing file's times to now, or creates a missing one unless noCreate.
func touchFile(native string, now time.Time, noCreate bool) error {
	err := os.Chtimes(native, now, now)
	switch {
	case err == nil:
		return nil
	case !errors.Is(err, fs.ErrNotExist):
		return err
	case noCreate:
		return nil
	}
	file, err := os.OpenFile(native, os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return err
	}
	return file.Close()
}

// mkdir takes -p and -m, the two options busybox's getopt32long string carries
// besides -v (coreutils/mkdir.c:63). Without option parsing the flags were
// taken as operands, so `mkdir -p a/b/c` created a directory literally named
// -p and then failed to create a/b/c because its parents were missing.
func newMkdirApplet() Applet {
	return simpleApplet{name: "mkdir", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, args, "pv", "m")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		view := ProcessViewFromContext(ctx)
		mode, err := mkdirMode(options, processFileModeMask(view))
		if err != nil {
			return err
		}
		made := true
		for _, path := range operands {
			native, err := resolveHostPath(view, path)
			if err != nil {
				return err
			}
			if err := makeDirectory(native, mode, options.has('p')); err != nil {
				// Of several operands, one that cannot be made is named and the rest made; see
				// operand_reporter.go.
				if len(operands) == 1 || !reportOperand(ctx, cannotCreateDirectory(path, err)) {
					return cannotCreateDirectory(path, err)
				}
				made = false
			}
		}
		if !made {
			return ExitStatus(1)
		}
		return nil
	}}
}

// mkdirMode is -m's MODE, which is chmod's, octal or symbolic, read against 777 as busybox's
// mkdir reads it (coreutils/mkdir.c:75).
func mkdirMode(options appletOptions, umask uint32) (os.FileMode, error) {
	if !options.has('m') {
		return 0o777, nil
	}
	parsed, ok := applyChmodMode(options.value('m'), 0o777, umask, false)
	if !ok {
		return 0, fmt.Errorf("invalid mode '%s'", options.value('m'))
	}
	return fileModeOfBits(parsed), nil
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
		options, operands, err := parseAppletOptions(ctx, args, "pv", "")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		view := ProcessViewFromContext(ctx)
		removed := true
		for _, path := range operands {
			if err := removeDirectoryTree(view, path, options.has('p')); err != nil {
				// Of several operands, one that cannot be removed is named and the rest removed; see
				// operand_reporter.go.
				if len(operands) == 1 || !reportOperand(ctx, err) {
					return err
				}
				removed = false
			}
		}
		if !removed {
			return ExitStatus(1)
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
