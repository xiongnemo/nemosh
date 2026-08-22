package applets

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// descendLsDirectories is -R: after a directory's own entries, each of its
// subdirectories in turn.
//
// The layout is busybox's, measured 2026-08-22: a blank line, then the
// directory's path and a colon, then its entries. There is no blank line after
// the last block, and an empty directory contributes a header and nothing else.
//
//	.:
//	only
//
//	./only:
//
// The header path is built from the operand *as spelled* rather than from the
// host path, so `ls -R sub` says `sub:` and `sub/nested:` -- the same rule find
// follows, and for the same reason: a script stripping the prefix it passed in
// must find it there.
func descendLsDirectories(stdout io.Writer, display string, items []lsEntry, options lsOptions) error {
	for _, item := range items {
		if !item.info.IsDir() || isLsDotEntry(item.name) {
			continue
		}
		if _, err := fmt.Fprintln(stdout); err != nil {
			return err
		}
		child := appendLsDisplayPath(display, item.name)
		if err := listDirectory(stdout, item.path, child, options, true); err != nil {
			return err
		}
	}
	return nil
}

// isLsDotEntry keeps -a from recursing for ever. `.` and `..` are listed,
// because -a asks for them and a directory's own mode is only visible through
// `.`, but following either one is a walk with no end.
func isLsDotEntry(name string) bool { return name == "." || name == ".." }

// appendLsDisplayPath joins a parent's spelling to a child's name.
//
// Not filepath.Join, which would clean the result: `ls -R .` must say `./sub`
// and not `sub`, exactly as `find .` writes `./a.txt`. A parent already ending
// in a separator does not get a second one.
func appendLsDisplayPath(parent, name string) string {
	slashed := filepath.ToSlash(parent)
	if slashed == "" {
		return name
	}
	if strings.HasSuffix(slashed, "/") || strings.HasSuffix(parent, `\`) {
		return parent + name
	}
	return parent + "/" + name
}
