//go:build windows

package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// openSharedTemp is a new temporary file, read and written through one handle, that
// Windows deletes once the last handle to it closes: the shell's, and every job's it was
// handed to.
func openSharedTemp() (*os.File, error) {
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return nil, err
	}
	path := filepath.Join(os.TempDir(), "nemosh-input-"+hex.EncodeToString(suffix))
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_TEMPORARY|windows.FILE_FLAG_DELETE_ON_CLOSE, 0)
	if err != nil {
		return nil, &os.PathError{Op: "create", Path: path, Err: err}
	}
	return os.NewFile(uintptr(handle), path), nil
}
