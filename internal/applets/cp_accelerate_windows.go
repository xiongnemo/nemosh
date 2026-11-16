package applets

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// A large regular file is copied by CopyFile2, which is what cmd's copy and Explorer copy with:
// it reads and writes in large unbuffered pieces, and a 512 MB file took 165 ms where reading and
// writing it here 32 KB at a time took 360 and busybox-w32's cp 950. CopyFile2 carries more than
// the bytes, so what it carried that cp does not is put back: the copy's time is now, as a new
// file's is, and its attributes are the archive bit and, for a source no one may write, read
// only, as cp's own copy makes them -- not the source's hidden or system bits. Its alternate data
// streams are left behind, as cp's copy of the bytes leaves them (COPY_FILE_SKIP_ALTERNATE_STREAMS,
// Windows 10 2004 and later; where it is refused the bytes are copied as before).

// cpAccelerateAt is the size from which a regular file is copied by CopyFile2.
var cpAccelerateAt int64 = 4 << 20

const (
	copyFileFailIfExists         = 0x1
	copyFileSkipAlternateStreams = 0x8000
	errorRequestAborted          = windows.Errno(1235)
	fileBasicInfoClass           = 0 // FileBasicInfo
)

var copyFile2Call struct {
	once sync.Once
	proc *windows.LazyProc
}

type copyFile2Parameters struct {
	size, flags uint32
	cancel      *int32
	progress    uintptr
	context     uintptr
}

// copyFileAccelerated copies source to dest, which must not be there, by CopyFile2, and ends the
// copy when ctx is cancelled.
func copyFileAccelerated(ctx context.Context, source, dest string) error {
	copyFile2Call.once.Do(func() { copyFile2Call.proc = windows.NewLazySystemDLL("kernel32.dll").NewProc("CopyFile2") })
	if err := copyFile2Call.proc.Find(); err != nil {
		return err
	}
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(dest)
	if err != nil {
		return err
	}
	cancel := new(int32)
	stop := context.AfterFunc(ctx, func() { atomic.StoreInt32(cancel, 1) })
	defer stop()
	parameters := copyFile2Parameters{flags: copyFileFailIfExists | copyFileSkipAlternateStreams, cancel: cancel}
	parameters.size = uint32(unsafe.Sizeof(parameters))
	result, _, _ := copyFile2Call.proc.Call(uintptr(unsafe.Pointer(from)), uintptr(unsafe.Pointer(to)), uintptr(unsafe.Pointer(&parameters)))
	if result == 0 {
		return nil
	}
	// An HRESULT from a Win32 error carries the error in its low word.
	if result>>16&0x1FFF == 7 {
		return windows.Errno(result & 0xFFFF)
	}
	return windows.Errno(result)
}

// settleCopiedFile gives a file CopyFile2 made what cp's own copy would have: the archive bit,
// read only when the source's owner may not write it, and unless the times are kept, now for its
// time.
func settleCopiedFile(path string, mode os.FileMode, keepTimes bool) error {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(name, windows.FILE_WRITE_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	var info struct {
		creation, access, write, change int64
		attributes                      uint32
		_                               uint32
	}
	info.attributes = windows.FILE_ATTRIBUTE_ARCHIVE
	if mode&0o200 == 0 {
		info.attributes |= windows.FILE_ATTRIBUTE_READONLY
	}
	if !keepTimes {
		now := windows.NsecToFiletime(time.Now().UnixNano())
		info.write = int64(now.HighDateTime)<<32 | int64(now.LowDateTime)
	}
	return windows.SetFileInformationByHandle(handle, fileBasicInfoClass, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
}

func isFileExists(err error) bool {
	return errors.Is(err, windows.ERROR_FILE_EXISTS) || errors.Is(err, windows.ERROR_ALREADY_EXISTS)
}
