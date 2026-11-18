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
//
// The size and the times are the handle's, which statHandleFacts opens whatever happens, and the
// FileInfo's only when no handle can be had: so a stat made from what a directory listing already
// says of an entry, hostStatRecordListed, is the same record, without the GetFileAttributesEx that
// was a third of `ls -l`'s time over System32.
func hostStatRecord(native string, follow bool, umask uint32) (statRecord, error) {
	info, err := lstatOf(native)
	if err == nil && follow {
		info, err = os.Stat(native)
	}
	if err != nil {
		return statRecord{}, err
	}
	return statRecordOf(native, info, nil, umask), nil
}

// lstatOf is how a stat asks GetFileAttributesEx of a path, which a test counts.
var lstatOf = os.Lstat

// hostStatRecordListed is hostStatRecord of an entry not followed, from what the listing that
// found it says of it: its kind, its attributes, whether it is a link. An entry whose handle
// cannot be had is asked GetFileAttributesEx for its size and times after all, since a listing
// gives a directory none.
func hostStatRecordListed(native string, umask uint32, listed os.FileInfo) (statRecord, error) {
	return statRecordOf(native, listed, func() (os.FileInfo, bool) {
		info, err := lstatOf(native)
		return info, err == nil
	}, umask), nil
}

// statRecordOf is the record of native, which info describes. fresh is how its size and times are
// had again when no handle tells them, or nil when info's are fresh already.
func statRecordOf(native string, info os.FileInfo, fresh func() (os.FileInfo, bool), umask uint32) statRecord {
	record := statRecord{size: info.Size(), blockSize: 4096, links: 1}
	setTimesOf(&record, info)
	target, link := linkTarget(native, info)
	switch {
	case link:
		record.mode, record.size, record.target = statLink|0o777, int64(len(target)), target
	case info.IsDir():
		record.mode, record.links = statDirectory|currentPermissions(native, info, umask), 2
	default:
		record.mode = statRegular | currentPermissions(native, info, umask)
	}
	// A link to a directory is counted as one, as busybox's count_subdirs counts whatever carries
	// the directory attribute: scoop's `current`, a junction, has its version's links.
	known := statHandleFacts(native, link, &record)
	if known && hasDirectoryAttribute(info) {
		record.links = subdirectoryLinks(native)
	}
	if !known && fresh != nil {
		if again, ok := fresh(); ok {
			setTimesOf(&record, again)
			if !link {
				record.size = again.Size()
			}
		}
	}
	size := record.size
	if held, ok := compressedSize(native, info); ok {
		size = held
	}
	record.blocks = (size + 4095) >> 12 << 3
	record.owner, record.group = busyboxAccountName(record.uid), busyboxAccountName(record.gid)
	return record
}

// setTimesOf is the record's times from info: access, modification, and creation for the change.
func setTimesOf(record *statRecord, info os.FileInfo) {
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		record.access = time.Unix(0, data.LastAccessTime.Nanoseconds())
		record.modify = time.Unix(0, data.LastWriteTime.Nanoseconds())
		record.change = time.Unix(0, data.CreationTime.Nanoseconds())
	}
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
		record.access = time.Unix(0, data.LastAccessTime.Nanoseconds())
		record.modify = time.Unix(0, data.LastWriteTime.Nanoseconds())
		record.change = time.Unix(0, data.CreationTime.Nanoseconds())
		if !link {
			// A link's size is its target's length, which the caller has set.
			record.size = int64(data.FileSizeHigh)<<32 | int64(data.FileSizeLow)
		}
	}
	uid, shared, account := fileOwner(handle)
	record.uid, record.gid, record.account = uid, uid, account
	if shared {
		record.mode |= (record.mode & 0o700) >> 6
	}
	return known
}

// hasDirectoryAttribute is FILE_ATTRIBUTE_DIRECTORY, which a directory carries and so does a
// junction or a link to one, where Go's IsDir says only the first.
func hasDirectoryAttribute(info os.FileInfo) bool {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return info.IsDir() || ok && data.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
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
	kindID, _ := windowsFilesystemTypes(windows.UTF16ToString(kind))
	return statfsRecord{
		id: uint64(serial) << 32, nameLength: uint64(nameLength), kind: kindID,
		blockSize: blockSize, blocks: total / blockSize, free: free / blockSize, available: available / blockSize,
	}, nil
}

// windowsFilesystemTypes is busybox-w32's FS_NAMES and fstypes (win32/statfs.c:16-17): the magic
// numbers of the filesystems Windows names, and 0 for the rest.
func windowsFilesystemTypes(key string) (uint64, bool) {
	switch key {
	case "NTFS":
		return 0x5346544e, true
	case "FAT", "FAT32":
		return 0x4006, true
	case "CDFS":
		return 0x9660, true
	case "UDF":
		return 0x15013346, true
	}
	var zero uint64
	return zero, false
}
