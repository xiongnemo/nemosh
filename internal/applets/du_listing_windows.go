//go:build windows

package applets

import (
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// fileIDBothDirInfo is FILE_ID_BOTH_DIR_INFO in its documented layout, which x/sys/windows does
// not declare: an entry of what GetFileInformationByHandleEx lists for FileIdBothDirectoryInfo,
// as Go's own os.ReadDir reads a directory.
type fileIDBothDirInfo struct {
	NextEntryOffset uint32
	FileIndex       uint32
	CreationTime    int64
	LastAccessTime  int64
	LastWriteTime   int64
	ChangeTime      int64
	EndOfFile       int64
	AllocationSize  int64
	FileAttributes  uint32
	FileNameLength  uint32
	EaSize          uint32
	ShortNameLength uint8
	ShortName       [12]uint16
	FileID          uint64
	FileName        [1]uint16
}

// duListingSize is what one call lists into; the buffer has room past it for an entry's
// declared size, so that the last one in it is still inside the buffer when read as a whole.
const duListingSize = 64 << 10

// readDuDirectory is a directory's entries in the order it lists them, a plain file's with what
// the listing says of it: its size, its attributes, and its index on the directory's volume.
// Where the listing cannot be read so, a filesystem that does not give it, every entry is left
// for walk to stat.
func readDuDirectory(native string) ([]duChild, error) {
	name, err := windows.UTF16PtrFromString(native)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: native, Err: err}
	}
	defer windows.CloseHandle(handle)
	var directory windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(handle, &directory) != nil {
		return unlistedChildren(native)
	}
	buffer := make([]uint64, (duListingSize+unsafe.Sizeof(fileIDBothDirInfo{}))/8+1)
	listing := unsafe.Slice((*byte)(unsafe.Pointer(&buffer[0])), len(buffer)*8)
	var children []duChild
	for {
		err := windows.GetFileInformationByHandleEx(handle, windows.FileIdBothDirectoryInfo, &listing[0], duListingSize)
		switch {
		case errors.Is(err, windows.ERROR_NO_MORE_FILES):
			return children, nil
		case err != nil && children == nil:
			return unlistedChildren(native)
		case err != nil:
			return children, &os.PathError{Op: "readdir", Path: native, Err: err}
		}
		for offset := 0; ; {
			entry := (*fileIDBothDirInfo)(unsafe.Pointer(&listing[offset]))
			text := windows.UTF16ToString(unsafe.Slice(&entry.FileName[0], entry.FileNameLength/2))
			if text != "." && text != ".." {
				children = append(children, listedChild(text, entry, uint64(directory.VolumeSerialNumber)))
			}
			if entry.NextEntryOffset == 0 {
				break
			}
			offset += int(entry.NextEntryOffset)
		}
	}
}

// listedChild is an entry as du takes it: listed, when it is a plain file.
func listedChild(name string, entry *fileIDBothDirInfo, device uint64) duChild {
	child := duChild{name: name}
	if entry.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT) == 0 {
		child.listed, child.size, child.attributes = true, entry.EndOfFile, entry.FileAttributes
		child.id = fileID{device: device, index: entry.FileID}
	}
	return child
}

// listedBlocks is du's size of a listed file, as size makes walk's of a stat'd one: bytes under
// -b; otherwise what it holds rounded up to the volume's cluster, a compressed or sparse file's
// held size asked for, in 512-byte blocks.
func listedBlocks(native string, child duChild, bytes bool) int64 {
	if bytes {
		return child.size
	}
	size := child.size
	if cluster, ok := volumeClusterSize(native); ok {
		if child.attributes&(windows.FILE_ATTRIBUTE_COMPRESSED|windows.FILE_ATTRIBUTE_SPARSE_FILE) != 0 {
			if held, ok := compressedFileSize(native); ok {
				size = held
			}
		}
		size = (size + cluster - 1) / cluster * cluster
	}
	return (size + 511) / 512
}
