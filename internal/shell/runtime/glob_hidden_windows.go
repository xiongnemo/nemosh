//go:build windows

package runtime

import (
	"io/fs"
	"syscall"
)

// fileAttributes reads the Hidden and System attributes from what the directory listing
// already fetched, so asking costs no second call per file.
func fileAttributes(entry fs.DirEntry) (hidden, system bool) {
	info, err := entry.Info()
	if err != nil {
		return false, false
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return false, false
	}
	return data.FileAttributes&syscall.FILE_ATTRIBUTE_HIDDEN != 0, data.FileAttributes&syscall.FILE_ATTRIBUTE_SYSTEM != 0
}
