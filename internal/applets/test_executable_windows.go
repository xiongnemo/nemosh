package applets

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// Windows has no execute permission bit, so busybox-w32 synthesises one (win32/mingw.c:778-784):
// a directory has it, and so does a regular file whose name ends in a suffix Windows runs, or
// whose first bytes say it is a program. mingw_access reads that bit for X_OK
// (win32/mingw.c:2131), so `test -x` answers from it, and chmod's and stat's modes show it.
//
// It went by the suffix alone, and knew four of the five: a #! script and foo.sh were not
// executable to test -x, though the shell ran both.
func isExecutableFile(native string, info os.FileInfo) bool {
	if info.IsDir() {
		return true
	}
	return info.Mode().IsRegular() && (executableSuffix(native) || executableFormat(native, info))
}

// executableSuffix is busybox-w32's has_exe_suffix (win32/mingw.c:2188-2213): after the name's
// last dot, a suffix of at most three characters, one of the five Windows runs, in any case.
func executableSuffix(name string) bool {
	suffix := filepath.Ext(name)
	if len(suffix) < 2 || len(suffix) > 4 {
		return false
	}
	switch strings.ToLower(suffix[1:]) {
	case "com", "exe", "sh", "bat", "cmd":
		return true
	}
	return false
}

// recallOnDataAccess is FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS: a file whose data is in the cloud,
// which reading would download, and which busybox therefore does not read to judge.
const recallOnDataAccess = 0x00400000

// executableFormat is has_exec_format's read: a DLL passed over by its name, as busybox passes
// over the thousands a system holds, and the rest judged by their first 1024 bytes.
func executableFormat(native string, info os.FileInfo) bool {
	if strings.EqualFold(filepath.Ext(native), ".dll") {
		return false
	}
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok && data.FileAttributes&recallOnDataAccess != 0 {
		return false
	}
	head, err := readHead(native)
	return err == nil && executableHeader(head)
}

// readHead is a file's first 1024 bytes, read through a handle that holds its access time, as
// busybox's does with SetFileTime and -1, so that asking what a file is does not make it look
// read. When that handle cannot be had the file is read as any is.
func readHead(native string) ([]byte, error) {
	var file *os.File
	name, err := windows.UTF16PtrFromString(native)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.FILE_WRITE_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, 0, 0)
	if err == nil {
		hold := windows.Filetime{LowDateTime: 0xffffffff, HighDateTime: 0xffffffff}
		windows.SetFileTime(handle, nil, &hold, nil)
		file = os.NewFile(uintptr(handle), native)
	} else if file, err = os.Open(native); err != nil {
		return nil, err
	}
	defer file.Close()
	head := make([]byte, 1024)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return head[:n], nil
}
