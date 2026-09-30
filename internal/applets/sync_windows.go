package applets

import (
	"errors"

	"golang.org/x/sys/windows"
)

// syncFilesystems is busybox-w32's mingw_sync: each volume that opens, `\\.\C:` and the rest, is
// flushed. Opening a volume to write takes an elevated session, so otherwise none opens, and a
// failure is no error, as it is not there.
func syncFilesystems() {
	for _, root := range windowsVolumeRoots() {
		rootName, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		if kind := windows.GetDriveType(rootName); kind != windows.DRIVE_FIXED && kind != windows.DRIVE_REMOVABLE {
			continue
		}
		name, err := windows.UTF16PtrFromString(`\\.\` + root[:2])
		if err != nil {
			continue
		}
		volume, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
		if err != nil {
			continue
		}
		_ = windows.FlushFileBuffers(volume)
		_ = windows.CloseHandle(volume)
	}
}

// isFlushRefusal is whether a flush failed only because its handle was opened to read, which
// busybox-w32's fsync calls success.
func isFlushRefusal(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED)
}
