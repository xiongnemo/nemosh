package applets

import (
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// accessTime is when the file was last read, which Windows keeps beside the modification time.
func accessTime(_ string, info os.FileInfo) time.Time {
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, data.LastAccessTime.Nanoseconds())
	}
	return info.ModTime()
}

// setLinkTimes sets a symbolic link's own times, opening the link itself rather than what it
// points at. A zero time is left as it is.
func setLinkTimes(native string, access, modified time.Time) error {
	name, err := windows.UTF16PtrFromString(native)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(name, windows.FILE_WRITE_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return &os.PathError{Op: "open", Path: native, Err: err}
	}
	defer windows.CloseHandle(handle)
	var accessTime, modifiedTime *windows.Filetime
	if !access.IsZero() {
		stamp := windows.NsecToFiletime(access.UnixNano())
		accessTime = &stamp
	}
	if !modified.IsZero() {
		stamp := windows.NsecToFiletime(modified.UnixNano())
		modifiedTime = &stamp
	}
	if err := windows.SetFileTime(handle, nil, accessTime, modifiedTime); err != nil {
		return &os.PathError{Op: "chtimes", Path: native, Err: err}
	}
	return nil
}
