package applets

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Removal reports every failure it meets and keeps going, because the useful
// answer to "why is this directory still here" is the whole list, not the first
// entry of it.
//
// Faithful to busybox's remove_file (libbb/remove_file.c:31-77): a directory's
// children are all attempted even after one of them fails, and the directory
// itself is only unlinked when every child succeeded. That second half is what
// keeps one locked file from producing a diagnostic for each directory above
// it -- a directory left non-empty is a consequence of a failure already
// reported, not a new failure. Measured against busybox, two held files under
// one tree produce exactly two lines.

// removeTree removes native and everything under it, and reports whether it
// succeeded. Diagnostics go to stderr as they happen rather than being
// collected, so a slow removal says what is wrong while it is still running.
//
// display is the path as it was written on the command line, carried alongside
// the resolved one. Naming the operand is not good enough: `cannot remove
// 'node_modules'` when a single file inside it is in use is a sentence the
// reader already knew, and it sends them to the wrong place. busybox names
// 'b/held.exe' and so does this.
func removeTree(native, display string, force bool, stderr io.Writer) bool {
	info, err := os.Lstat(native)
	if err != nil {
		if force && errors.Is(err, fs.ErrNotExist) {
			return true
		}
		reportRemoveFailure(stderr, display, err)
		return false
	}
	// A symlink is removed, never followed -- Lstat rather than Stat is what
	// keeps `rm -rf link-to-home` from emptying the target.
	if !info.IsDir() {
		return removeOne(native, display, stderr)
	}
	entries, err := os.ReadDir(native)
	if err != nil {
		reportRemoveFailure(stderr, display, err)
		return false
	}
	removed := true
	for _, entry := range entries {
		if !removeTree(filepath.Join(native, entry.Name()), display+"/"+entry.Name(), force, stderr) {
			removed = false
		}
	}
	if !removed {
		// Deliberately silent: the directory is still here because of something
		// already reported above.
		return false
	}
	return removeOne(native, display, stderr)
}

func removeOne(native, display string, stderr io.Writer) bool {
	if err := os.Remove(native); err != nil {
		reportRemoveFailure(stderr, display, err)
		return false
	}
	return true
}

// The wording is the applet's own rather than the shell's, because the shell
// only prints a diagnostic for an error an applet returns, and an applet that
// reports several has to return a bare status instead. Routing through
// cannotRemove keeps the sentence identical to the one the shell would have
// printed.
func reportRemoveFailure(stderr io.Writer, display string, err error) {
	fmt.Fprintf(stderr, "rm: %v\n", cannotRemove(display, err))
}
