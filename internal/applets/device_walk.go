package applets

import (
	"io/fs"
)

// Walking `/dev`, for the three applets that walk.
//
// Stage 3 of docs/design/device-filesystem.md, and smaller than that document expected because a
// measurement moved it. The plan assumed `find /` would meet `/dev` as it does on Linux, and that
// the walkers would therefore need an `fs.FS` spanning the real filesystem and the synthetic one.
// They do not: `/` resolves to `/c` here, the current drive's root, so `/dev` is a *sibling*
// top-level name rather than a directory inside `/`. Measured -- `ls / | grep -c '^dev$'` is zero --
// and it removes the splicing, the per-entry path translation and the performance question with it.
//
// So what is needed is not a filesystem interface but a walk of one directory that has no
// subdirectories. Building the general thing for a single one-level tree that no real walk can reach
// would be the abstraction-for-one-tenant mistake the design document warns about; it stays on the
// shelf until `/proc` asks for it, which is when it starts paying.

// walkDeviceRoot visits a device path and everything under it, and reports whether it handled the
// path at all.
//
// `false, nil` means "not a device path, walk it yourself", which keeps every caller's ordinary
// filesystem walk exactly as it was.
//
// The visit order is the operand first and then its contents, which is what `filepath.WalkDir` does
// and therefore what the callers already expect: `find /dev` prints `/dev` before `/dev/null`.
func walkDeviceRoot(view ProcessView, root string, visit func(path string, entry fs.DirEntry) error) (bool, error) {
	info, err := statDeviceOperand(view, root)
	if err != nil || info == nil {
		return false, err
	}
	if !info.IsDir() {
		// A device named directly rather than a directory: one entry, itself.
		return true, visit(root, deviceWalkEntry{name: info.Name(), info: info})
	}
	if err := visit(root, deviceWalkEntry{name: info.Name(), info: info}); err != nil {
		return true, err
	}
	entries, err := readDirProcessPath(view, root)
	if err != nil {
		return true, err
	}
	for _, entry := range entries {
		child := root + "/" + entry.Name()
		if root == "/" {
			child = root + entry.Name()
		}
		if err := visit(child, entry); err != nil {
			return true, err
		}
	}
	return true, nil
}

// deviceWalkEntry adapts a FileInfo to the DirEntry a walk callback is given.
//
// The callbacks in find, du and grep all take fs.DirEntry, so handing them one keeps the device
// case and the filesystem case going through the same predicate code -- which is what makes
// `find /dev -type c` work without find knowing anything about devices.
type deviceWalkEntry struct {
	name string
	info fs.FileInfo
}

func (e deviceWalkEntry) Name() string               { return e.name }
func (e deviceWalkEntry) IsDir() bool                { return e.info.IsDir() }
func (e deviceWalkEntry) Type() fs.FileMode          { return e.info.Mode().Type() }
func (e deviceWalkEntry) Info() (fs.FileInfo, error) { return e.info, nil }
