package applets

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Copying a directory is the one thing `cp` could not do, and there is no way
// round it: no combination of the other applets copies a tree.
//
// The shape is busybox's, measured rather than assumed. `cp src dst` with src a
// directory and no -r answers `cp: omitting directory 'src'` and exits 1. With
// -r and a destination that does not exist, dst *becomes* the copy of src; with
// a destination that already exists as a directory, the copy lands at dst/src.
// That second rule is copyDestination's, which cp already applied to files, so
// recursion only had to reuse it.

// omittingDirectory is the refusal when -r was not given. busybox words it with
// no errno attached, because there is no failed call to report -- the operand
// was simply not the kind this can take.
func omittingDirectory(operand string) error {
	return fmt.Errorf("omitting directory '%s'", operand)
}

// copyTree copies source onto dest, which must not exist yet as a file.
//
// A symlink is copied by following it rather than by recreating it. That is a
// deliberate simplification for Windows, where creating one needs a privilege an
// ordinary session does not have, and where a copy that fails is worse than a
// copy that is slightly less faithful. POSIX gives -r that licence: it is -R
// that must preserve, and this shell implements -r.
func copyTree(source, dest pathOperand, stderr io.Writer) error {
	walk := &copyTreeWalk{stderr: stderr}
	if err := walk.copy(source, dest); err != nil {
		return err
	}
	if walk.omitted {
		// The diagnostics are already out; this only carries the status.
		return ErrExitFalse
	}
	return nil
}

// copyTreeWalk carries what a recursive copy has to know beyond the pair of paths in front
// of it: where the whole copy is being written, and whether anything was skipped.
type copyTreeWalk struct {
	stderr io.Writer
	// root is the destination the top-level copy created, empty until it exists.
	root string
	// omitted records that a subtree was skipped, which is a failure for the exit status
	// even though the copy went on.
	omitted bool
}

// copy copies one node, recursing into a directory.
//
// root is what stops `cp -r . sub` from running until the disk is full. The destination is
// created inside the source, so reading the source again finds it, copies it, finds the copy
// it just made, and so on: measured before this, it built `sub/sub/sub/...` several hundred
// levels deep and was still going. busybox detects the same thing, answers
// `cp: recursion detected, omitting directory 'X'`, copies everything else, and exits 1 --
// all three of which this now does. GNU words it differently and also exits 1.
func (walk *copyTreeWalk) copy(source, dest pathOperand) error {
	info, err := os.Lstat(source.host)
	if err != nil {
		return cannotStat(source.operand, err)
	}
	if !info.IsDir() {
		return copyFile(source, dest)
	}
	if walk.root != "" && pathWithin(source.host, walk.root) {
		walk.omitted = true
		_, err := fmt.Fprintf(walk.stderr, "cp: recursion detected, omitting directory '%s'\n", source.operand)
		return err
	}
	if err := os.MkdirAll(dest.host, directoryPermissions(info)); err != nil {
		return cannotCreateDirectory(dest.operand, err)
	}
	if walk.root == "" {
		walk.root = dest.host
	}
	entries, err := os.ReadDir(source.host)
	if err != nil {
		return cannotOpen(source.operand, err)
	}
	for _, entry := range entries {
		child := entry.Name()
		next := pathOperand{
			host:    filepath.Join(source.host, child),
			operand: filepath.ToSlash(filepath.Join(source.operand, child)),
		}
		into := pathOperand{
			host:    filepath.Join(dest.host, child),
			operand: filepath.ToSlash(filepath.Join(dest.operand, child)),
		}
		if err := walk.copy(next, into); err != nil {
			return err
		}
	}
	return nil
}

// pathWithin reports whether path is root or sits under it.
//
// Compared after Clean and with the separator appended, so `sub2` is not read as being inside
// `sub`. Case-insensitively on Windows, where `SUB` and `sub` are the same directory and a
// case-sensitive test would miss the recursion it exists to catch.
func pathWithin(path, root string) bool {
	path, root = filepath.Clean(path), filepath.Clean(root)
	if samePathText(path, root) {
		return true
	}
	prefix := root + string(filepath.Separator)
	return len(path) > len(prefix) && samePathText(path[:len(prefix)], prefix)
}

// directoryPermissions keeps the source's mode where the platform has one, and
// falls back to something a user can write into. Windows reports 0777 for every
// directory, so this is a no-op there and matters only on the build-and-test
// platforms.
func directoryPermissions(info fs.FileInfo) fs.FileMode {
	if mode := info.Mode().Perm(); mode != 0 {
		return mode
	}
	return 0o755
}

// samePathText compares two path spellings, case-insensitively on Windows where `SUB` and
// `sub` name the same directory and a case-sensitive test would miss the recursion pathWithin
// exists to catch.
func samePathText(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
