package applets

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

// touchOpen is how touch opens a file to set its times; a test counts the opens.
var touchOpen = windows.CreateFile

// apply sets native's times in one open of it and one SetFileTime: OPEN_ALWAYS, which makes the
// file if it is not there, with perm's read-only bit, or OPEN_EXISTING under noCreate, which then
// passes over a file that is not there. With noDereference the open is of a symbolic link itself
// (FILE_FLAG_OPEN_REPARSE_POINT), whose own times are then set; without it, of what it points
// at, as utimes follows a link. A directory is opened as one with FILE_FLAG_BACKUP_SEMANTICS. A
// zero time is left as it is.
//
// touch stat-ed each FILE, opened it to set its times, and for one that was not there opened it
// again to make it and a third time to set times given with -d, -t or -r.
func (s touchStamp) apply(native string, perm os.FileMode, noCreate, noDereference bool) error {
	name, err := windows.UTF16PtrFromString(longPathFor(native))
	if err != nil {
		return err
	}
	disposition := uint32(windows.OPEN_ALWAYS)
	if noCreate {
		disposition = windows.OPEN_EXISTING
	}
	flags := uint32(windows.FILE_FLAG_BACKUP_SEMANTICS)
	if noDereference {
		flags |= windows.FILE_FLAG_OPEN_REPARSE_POINT
	}
	if perm&0o200 == 0 {
		flags |= windows.FILE_ATTRIBUTE_READONLY
	}
	handle, err := touchOpen(name, windows.FILE_WRITE_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, disposition, flags, 0)
	if err != nil {
		if noCreate && errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return &os.PathError{Op: "open", Path: native, Err: err}
	}
	defer windows.CloseHandle(handle)
	var accessTime, modifiedTime *windows.Filetime
	if !s.access.IsZero() {
		stamp := windows.NsecToFiletime(s.access.UnixNano())
		accessTime = &stamp
	}
	if !s.modified.IsZero() {
		stamp := windows.NsecToFiletime(s.modified.UnixNano())
		modifiedTime = &stamp
	}
	if err := windows.SetFileTime(handle, nil, accessTime, modifiedTime); err != nil {
		return &os.PathError{Op: "chtimes", Path: native, Err: err}
	}
	return nil
}

// longPathFor is native as CreateFileW takes a path past MAX_PATH, `\\?\` and whole, which os's
// own calls do for themselves.
func longPathFor(native string) string {
	if len(native) < 248 || strings.HasPrefix(native, `\\?\`) {
		return native
	}
	whole, err := filepath.Abs(native)
	if err != nil {
		return native
	}
	if strings.HasPrefix(whole, `\\`) {
		return `\\?\UNC\` + whole[2:]
	}
	return `\\?\` + whole
}
