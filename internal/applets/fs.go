package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// mkdir is busybox's (coreutils/mkdir.c and libbb/make_directory.c): `mkdir [-pv] [-m MODE]
// DIRECTORY...`. -p makes every missing parent and takes a directory that is already there;
// -m sets the mode of the last one to MODE exactly, as chmod reads MODE; -v says `created
// directory: 'x'` for each one made, a parent's name with the slash it ends at. --parents,
// --mode and --verbose stand for their letters.
//
// -v was read and said nothing. Without option parsing the flags were once taken as operands,
// so `mkdir -p a/b/c` created a directory literally named -p.
func newMkdirApplet() Applet {
	return simpleApplet{name: "mkdir", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout io.Writer, _ io.Writer) error {
		options, operands, err := parseAppletLongOptions(ctx, args, map[string]string{"parents": "p", "verbose": "v", "mode": "m"}, "pv", "m")
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
			if err := makeDirectory(view, path, options, mode, stdout); err != nil {
				// Of several operands, one that cannot be made is named and the rest made; see
				// operand_reporter.go.
				if len(operands) == 1 || !reportOperand(ctx, err) {
					return err
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

// makeDirectory is bb_make_directory: with -p each missing parent is made in turn, a name ending
// at the slash after it, and one that is already a directory is passed over. A failure names the
// step it came at, `f/` of `f/x`, as busybox's does. `/` and `.` have nothing to make.
func makeDirectory(view ProcessView, path string, options appletOptions, mode os.FileMode, stdout io.Writer) error {
	if path == "/" || path == "." {
		return nil
	}
	display := filepath.ToSlash(path)
	steps := []string{display}
	if options.has('p') {
		steps = directoryPrefixes(display)
	}
	for index, step := range steps {
		native, err := resolveHostPath(view, step)
		if err != nil {
			return err
		}
		last := index == len(steps)-1
		if err := makeOneDirectory(native, createMode(view, 0o777)); err != nil {
			if !options.has('p') || !errors.Is(err, fs.ErrExist) {
				return cannotCreateDirectory(step, err)
			}
			switch info, statErr := os.Stat(native); {
			case statErr == nil && info.IsDir():
				if last {
					return nil
				}
				continue
			// stat(2) says `f/` of a file is no directory, where Windows calls the name invalid.
			case strings.HasSuffix(step, "/") && !errors.Is(statErr, fs.ErrNotExist):
				return cannotCreateDirectory(step, errNotADirectory)
			case statErr != nil:
				return cannotCreateDirectory(step, statErr)
			}
			return cannotCreateDirectory(step, err)
		}
		if options.has('v') {
			fmt.Fprintf(stdout, "created directory: '%s'\n", step)
		}
		if last && options.has('m') {
			if err := applyPermissions(native, bitsOfFileMode(mode), true); err != nil {
				return fmt.Errorf("cannot set permissions of directory '%s': %s", step, causeText(err))
			}
		}
	}
	return nil
}

// makeOneDirectory is mkdir(2) as busybox-w32's mingw_mkdir has it: Windows refuses a drive's
// root, `/c/`, with ERROR_ACCESS_DENIED, and a refusal of something already there is EEXIST.
// perm is 0777 through the umask, as busybox's mkdir makes one.
func makeOneDirectory(native string, perm os.FileMode) error {
	err := os.Mkdir(native, perm)
	if errors.Is(err, fs.ErrPermission) {
		if _, statErr := os.Stat(native); statErr == nil {
			return &fs.PathError{Op: "mkdir", Path: native, Err: fs.ErrExist}
		}
	}
	return err
}

// directoryPrefixes are the names -p makes in turn: `b/c/d` is `b/`, `b/c/` and `b/c/d`. A root,
// `/` or `C:/`, is no step of its own.
func directoryPrefixes(path string) []string {
	var prefixes []string
	start := len(filepath.VolumeName(path))
	for start < len(path) && path[start] == '/' {
		start++
	}
	for index := start; index < len(path); index++ {
		if path[index] != '/' {
			continue
		}
		end := index
		for end < len(path) && path[end] == '/' {
			end++
		}
		if end < len(path) {
			prefixes = append(prefixes, path[:end])
		}
		index = end - 1
	}
	return append(prefixes, path)
}

// rmdir is busybox's (coreutils/rmdir.c): `rmdir [-pv] DIRECTORY...` removes each empty
// DIRECTORY, and with -p each parent after it that is left empty. -v says `rmdir: removing
// directory, 'x'` before each; --ignore-fail-on-non-empty passes over a DIRECTORY that is not
// empty without a word; --parents and --verbose stand for their letters.
//
// -v was read and said nothing, and --ignore-fail-on-non-empty, which Debian's packages lean on,
// was refused.
//
// rmdir removes directories and nothing else. It used to call os.Remove, which also removes
// files, so `rmdir notes.txt` deleted the file and reported success -- silent data loss against
// rmdir(3), which fails with ENOTDIR (coreutils/rmdir.c:73).
func newRmdirApplet() Applet {
	return simpleApplet{name: "rmdir", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout io.Writer, _ io.Writer) error {
		ignoreNonEmpty := false
		var words []string
		for index, arg := range args {
			if arg == "--" {
				words = append(words, args[index:]...)
				break
			}
			if arg == "--ignore-fail-on-non-empty" {
				ignoreNonEmpty = true
				continue
			}
			words = append(words, arg)
		}
		options, operands, err := parseAppletLongOptions(ctx, words, map[string]string{"parents": "p", "verbose": "v"}, "pv", "")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		view := ProcessViewFromContext(ctx)
		removed := true
		for _, path := range operands {
			for {
				if options.has('v') {
					fmt.Fprintf(stdout, "rmdir: removing directory, '%s'\n", path)
				}
				// A path the shell's view refuses, a disabled /cygdrive, is returned as it is.
				native, err := resolveHostPath(view, path)
				if err != nil {
					return err
				}
				err = removeEmptyDirectory(native)
				if err != nil && !(ignoreNonEmpty && isNotEmpty(err)) {
					// Of several operands, one that cannot be removed is named and the rest removed;
					// see operand_reporter.go.
					if len(operands) == 1 || !reportOperand(ctx, quotedFailure(path, err)) {
						return quotedFailure(path, err)
					}
					removed = false
				}
				// The parent as dirname spells it, with the separators path was written with.
				parent := dirnameOf(path)
				if err != nil || !options.has('p') || parent == path || parent == "." || parent == "/" {
					break
				}
				path = parent
			}
		}
		if !removed {
			return ExitStatus(1)
		}
		return nil
	}}
}

// removeEmptyDirectory removes path if it is a directory, which the stat a moment before makes
// sure of: it is a race Go's portable API leaves no way to close, but the ordinary case of
// naming a file by mistake cannot destroy it.
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

// isNotEmpty is whether err says a directory still holds something.
func isNotEmpty(err error) bool {
	return errors.Is(err, syscall.ENOTEMPTY) || causeText(err) == "Directory not empty"
}

// isNotThere is whether err says a file is not there, as busybox-w32's stat has it. Asking
// Windows about a name it cannot hold, a glob that matched nothing such as `*.tmp`, fails as an
// invalid name, which get_file_attr counts as not there with most other failures.
func isNotThere(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || causeText(err) == "No such file or directory"
}
