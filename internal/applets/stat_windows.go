package applets

import (
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// hostStatRecord is busybox-w32's do_lstat (win32/mingw.c:730-846). A link is one when Windows
// can say what it holds, a symbolic link or a junction: its mode is 0777, and its size is the
// length of its target. Anything else has the mode currentPermissions makes up. The change
// time is the creation time, as Windows keeps no change time; the blocks are 512-byte ones, in
// whole 4096-byte blocks of what the file holds.
func hostStatRecord(native string, follow bool, umask uint32) (statRecord, error) {
	info, err := os.Lstat(native)
	if err == nil && follow {
		info, err = os.Stat(native)
	}
	if err != nil {
		return statRecord{}, err
	}
	record := statRecord{size: info.Size(), blockSize: 4096, links: 1}
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		record.access = time.Unix(0, data.LastAccessTime.Nanoseconds())
		record.modify = time.Unix(0, data.LastWriteTime.Nanoseconds())
		record.change = time.Unix(0, data.CreationTime.Nanoseconds())
	}
	target, link := linkTarget(native, info)
	switch {
	case link:
		record.mode, record.size, record.target = statLink|0o777, int64(len(target)), target
	case info.IsDir():
		record.mode, record.links = statDirectory|currentPermissions(native, info, umask), 2
	default:
		record.mode = statRegular | currentPermissions(native, info, umask)
	}
	if statHandleFacts(native, link, &record) && info.IsDir() && !link {
		record.links = subdirectoryLinks(native)
	}
	size := record.size
	if held, ok := compressedSize(native, info); ok {
		size = held
	}
	record.blocks = (size + 4095) >> 12 << 3
	record.owner, record.group = busyboxAccountName(record.uid), busyboxAccountName(record.gid)
	return record, nil
}

// statHandleFacts is what busybox-w32's lstat asks of the file opened (win32/mingw.c:793-822):
// the volume's serial number for the device, the file's index on it for the inode, its count
// of links, and its owner. One it cannot open is owned by 0, and gives others nothing.
func statHandleFacts(native string, link bool, record *statRecord) bool {
	flags := uint32(windows.FILE_FLAG_BACKUP_SEMANTICS)
	if link {
		flags |= windows.FILE_FLAG_OPEN_REPARSE_POINT
	}
	name, err := windows.UTF16PtrFromString(native)
	if err != nil {
		return false
	}
	handle, err := windows.CreateFile(name, windows.READ_CONTROL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, flags, 0)
	if err != nil {
		record.mode &^= 0o007
		return false
	}
	defer windows.CloseHandle(handle)
	var data windows.ByHandleFileInformation
	known := windows.GetFileInformationByHandle(handle, &data) == nil
	if known {
		record.device = uint64(data.VolumeSerialNumber)
		record.inode = uint64(data.FileIndexHigh)<<32 | uint64(data.FileIndexLow)
		record.links = uint64(data.NumberOfLinks)
	}
	uid, shared := fileOwner(handle)
	record.uid, record.gid = uid, uid
	if shared {
		record.mode |= (record.mode & 0o700) >> 6
	}
	return known
}

// subdirectoryLinks is busybox-w32's count_subdirs (win32/mingw.c:704-720), which its stat asks
// for: the entries of a directory that are directories, links to one not among them, and . and
// .., which its readdir makes up where Windows lists neither (win32/dirent.c:89-99).
func subdirectoryLinks(native string) uint64 {
	entries, err := os.ReadDir(native)
	if err != nil {
		return 2
	}
	count := uint64(2)
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	return count
}

// hostStatfs is busybox-w32's statfs (win32/statfs.c): the volume a FILE is on, its sizes in
// blocks of a size made up from the volume's size, since Windows will not say, its serial
// number for an ID, and a type from the name Windows gives its filesystem. It counts no inodes.
func hostStatfs(native string) (statfsRecord, error) {
	if _, err := os.Stat(native); err != nil {
		return statfsRecord{}, err
	}
	name, err := windows.UTF16PtrFromString(native)
	if err != nil {
		return statfsRecord{}, err
	}
	root := make([]uint16, windows.MAX_LONG_PATH)
	if err := windows.GetVolumePathName(name, &root[0], uint32(len(root))); err != nil {
		return statfsRecord{}, err
	}
	var available, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(&root[0], &available, &total, &free); err != nil {
		return statfsRecord{}, err
	}
	var serial, nameLength, flags uint32
	kind := make([]uint16, 100)
	if err := windows.GetVolumeInformation(&root[0], nil, 0, &serial, &nameLength, &flags, &kind[0], uint32(len(kind))); err != nil {
		return statfsRecord{}, err
	}
	blockSize := uint64(4096)
	for limit := uint64(16) << 40; blockSize < 65536 && total >= limit; limit *= 2 {
		blockSize *= 2
	}
	return statfsRecord{
		id: uint64(serial) << 32, nameLength: uint64(nameLength), kind: windowsFilesystemTypes[windows.UTF16ToString(kind)],
		blockSize: blockSize, blocks: total / blockSize, free: free / blockSize, available: available / blockSize,
	}, nil
}

// windowsFilesystemTypes is busybox-w32's FS_NAMES and fstypes (win32/statfs.c:16-17): the magic
// numbers of the filesystems Windows names, and 0 for the rest.
var windowsFilesystemTypes = map[string]uint64{"NTFS": 0x5346544e, "FAT": 0x4006, "FAT32": 0x4006, "CDFS": 0x9660, "UDF": 0x15013346}
