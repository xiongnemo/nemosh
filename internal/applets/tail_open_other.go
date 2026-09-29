//go:build !windows

package applets

import (
	"io/fs"
	"os"
	"syscall"
)

// openShared opens a FILE tail reads. Here a file open for reading can always be renamed or
// removed under it; only a directory is refused, as the other applets refuse one.
func openShared(native string) (*os.File, error) {
	file, err := os.Open(native)
	if err != nil {
		return nil, err
	}
	if info, err := file.Stat(); err == nil && info.IsDir() {
		file.Close()
		return nil, &fs.PathError{Op: "open", Path: native, Err: syscall.EISDIR}
	}
	return file, nil
}
