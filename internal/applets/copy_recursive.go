package applets

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
func copyTree(source, dest pathOperand) error {
	info, err := os.Lstat(source.host)
	if err != nil {
		return cannotStat(source.operand, err)
	}
	if !info.IsDir() {
		return copyFile(source, dest)
	}
	if err := os.MkdirAll(dest.host, directoryPermissions(info)); err != nil {
		return cannotCreateDirectory(dest.operand, err)
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
		if err := copyTree(next, into); err != nil {
			return err
		}
	}
	return nil
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
