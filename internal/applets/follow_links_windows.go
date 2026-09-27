//go:build windows

package applets

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// FollowLinks is path with every link in it followed: symbolic links, and junctions as well,
// which is how busybox-w32 resolves a path. filepath.EvalSymlinks leaves a junction as it is
// on Windows, so `realpath` through one answered the junction, where busybox's answers the
// directory it leads to. EvalSymlinks goes first, component by component, which keeps `..`
// after a symbolic link meaning the link's target's parent; what is left of a junction is
// then asked of the file's handle.
//
// Only a path with a junction left in it is asked about. Asked of every path, a mapped drive
// would come back as the share it maps, which nothing here asked for.
func FollowLinks(path string) (string, error) {
	evaluated, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	if !throughJunction(evaluated) {
		return evaluated, nil
	}
	if final, ok := finalPath(evaluated); ok {
		return final, nil
	}
	return evaluated, nil
}

// throughJunction reports whether a component of path is a reparse point that is not a
// symbolic link, which is how a junction looks to os.Lstat.
func throughJunction(path string) bool {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		if info, err := os.Lstat(current); err == nil && info.Mode()&os.ModeIrregular != 0 {
			return true
		}
		if parent := filepath.Dir(current); parent == current {
			return false
		}
	}
}

// finalPath is the path the filesystem opens for path, without the `\\?\` it answers with.
func finalPath(path string) (string, bool) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", false
	}
	handle, err := windows.CreateFile(name, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", false
	}
	defer windows.CloseHandle(handle)
	buffer := make([]uint16, windows.MAX_PATH)
	for {
		length, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
		if err != nil {
			return "", false
		}
		if int(length) < len(buffer) {
			break
		}
		buffer = make([]uint16, length+1)
	}
	final := windows.UTF16ToString(buffer)
	switch {
	case strings.HasPrefix(final, `\\?\UNC\`):
		return `\\` + final[len(`\\?\UNC\`):], true
	case strings.HasPrefix(final, `\\?\`):
		return final[len(`\\?\`):], true
	}
	return final, true
}
