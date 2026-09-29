//go:build windows

package applets

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// What `os.FileInfo` does not carry, and what `du` and `ls -l` need from Windows directly.
//
// No new dependency: `golang.org/x/sys/windows` is already linked, for `id` and for the
// console work in cmd/nemosh. What it costs is calls -- see each function.

// allocatedBytes is how much room an entry du has stat'd occupies on disk, in bytes.
//
// Not FILE_STANDARD_INFO's AllocationSize, which was the obvious answer and the wrong one:
// NTFS keeps a small file *inside* its MFT record and reports an allocation of zero for it, so
// a one-byte file came back as occupying nothing. busybox-w32 rounds the size up to the
// cluster instead and answers 4K, which is both what a user means by disk usage and what the
// primary reference prints. Measured: a 1-byte, a 5-byte and a 1500-byte file all cost 4K
// there, and this now agrees on all three.
//
// A directory is rounded the same way. Its entries live in its MFT record until there are too
// many for it, when Windows reports a size for it -- 4096 for one of eight entries, measured --
// and busybox counts that as it counts a file's. It said 0 for every directory, which is right
// only for a small one. A link's size is the length of its text, entrySize.
//
// The cost is one GetDiskFreeSpace per volume, cached, and no per-file call, except for a
// file that is compressed or sparse: that one is what GetCompressedFileSize says it holds, as
// busybox-w32 asks for it too.
func allocatedBytes(path string, info os.FileInfo) (int64, bool) {
	cluster, ok := volumeClusterSize(path)
	if !ok {
		return 0, false
	}
	size, ok := compressedSize(path, info)
	if !ok {
		size = entrySize(path, info)
	}
	return (size + cluster - 1) / cluster * cluster, true
}

var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procGetDiskFreeSpace      = kernel32.NewProc("GetDiskFreeSpaceW")
	procGetCompressedFileSize = kernel32.NewProc("GetCompressedFileSizeW")
)

// compressedSize is what a compressed or sparse file holds on disk. Only such a file costs the
// call; the attributes that say so came with its stat.
func compressedSize(path string, info os.FileInfo) (int64, bool) {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok || !info.Mode().IsRegular() ||
		data.FileAttributes&(windows.FILE_ATTRIBUTE_COMPRESSED|windows.FILE_ATTRIBUTE_SPARSE_FILE) == 0 {
		return 0, false
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, false
	}
	var high uint32
	low, _, callErr := procGetCompressedFileSize.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&high)))
	if uint32(low) == 0xFFFFFFFF && callErr != windows.ERROR_SUCCESS {
		return 0, false
	}
	return int64(high)<<32 | int64(uint32(low)), true
}

// clusterSizes caches the answer per volume root, because `du` on a tree asks once per file
// and the answer cannot change under it.
var clusterSizes sync.Map

func volumeClusterSize(path string) (int64, bool) {
	// filepath.Abs only when the path does not already name a volume. It calls Getwd,
	// which is a syscall, and `du` asks once per file: on a 4,895-entry tree that alone
	// cost around 20ms of the 150 the walk took.
	volume := filepath.VolumeName(path)
	if volume == "" {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return 0, false
		}
		volume = filepath.VolumeName(absolute)
	}
	root := volume + string(filepath.Separator)
	if cached, ok := clusterSizes.Load(root); ok {
		return cached.(int64), true
	}
	name, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return 0, false
	}
	// Declared here because x/sys/windows wraps only GetDiskFreeSpaceEx, which reports
	// free and total bytes and not the cluster size this needs.
	var sectorsPerCluster, bytesPerSector, freeClusters, totalClusters uint32
	result, _, _ := procGetDiskFreeSpace.Call(
		uintptr(unsafe.Pointer(name)),
		uintptr(unsafe.Pointer(&sectorsPerCluster)),
		uintptr(unsafe.Pointer(&bytesPerSector)),
		uintptr(unsafe.Pointer(&freeClusters)),
		uintptr(unsafe.Pointer(&totalClusters)),
	)
	if result == 0 {
		return 0, false
	}
	cluster := int64(sectorsPerCluster) * int64(bytesPerSector)
	if cluster <= 0 {
		return 0, false
	}
	clusterSizes.Store(root, cluster)
	return cluster, true
}

// fileStandardInfo is FILE_STANDARD_INFO. x/sys/windows declares the class constant and the
// call but not the structure, so it is declared here in its documented layout.
type fileStandardInfo struct {
	AllocationSize int64
	EndOfFile      int64
	NumberOfLinks  uint32
	DeletePending  bool
	Directory      bool
}

// fileLinkCount is how many hard links point at the file. Windows has them, and `ls -l`
// prints the count.
//
// This one does cost a handle open and a call per file, which is why it is only reached from
// `ls -l` and only for the long form.
func fileLinkCount(path string) (int, bool) {
	handle, ok := openForMetadata(path, 0)
	if !ok {
		return 0, false
	}
	defer windows.CloseHandle(handle)
	var info fileStandardInfo
	if err := windows.GetFileInformationByHandleEx(handle, windows.FileStandardInfo,
		(*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		return 0, false
	}
	return int(info.NumberOfLinks), true
}

// openForMetadata opens a path for asking questions about it and nothing else.
//
// No access rights beyond metadata, so a file another process holds open for writing can
// still be asked; FILE_FLAG_BACKUP_SEMANTICS so a directory can be opened too. flags adds to
// that: FILE_FLAG_OPEN_REPARSE_POINT opens a link as itself rather than what it points at.
func openForMetadata(path string, flags uint32) (windows.Handle, bool) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, false
	}
	handle, err := windows.CreateFile(name, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|flags, 0)
	if err != nil {
		return 0, false
	}
	return handle, true
}

// fileOwnerName is the account that owns the file, mapped the way busybox-w32 maps it.
//
// The rule is measured from two observations: a file owned by my account prints `nemo`, and one
// owned by NT SERVICE\TrustedInstaller prints `root`. So a real user account gives its name and
// anything else -- a service identity, Administrators, SYSTEM -- gives `root`, which is
// busybox's uid-0 emulation and the same rule currentUserID already applies to the process.
//
// A failure gives `root` too, because a listing that stops because one file's owner could not
// be read would be worse than a listing with one pessimistic column.
func fileOwnerName(path string) string {
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return "root"
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return "root"
	}
	return cachedOwnerName(owner.String(), func() string {
		account, _, kind, err := owner.LookupAccount("")
		if err != nil || kind != windows.SidTypeUser {
			return "root"
		}
		return account
	})
}

// isSymbolicLink reports whether the entry is a link of some kind.
//
// Not `info.Mode()&os.ModeSymlink`, which is false for the ones that matter here. A Windows
// *junction* is a reparse point rather than a symlink, and Go reports it as ModeIrregular --
// so `ls -l` printed `?rw-rw-rw-` for the ten junctions in a home directory, where busybox
// prints `lrwxrwxrwx` and the target. The attribute is what both cases have in common.
func isSymbolicLink(info os.FileInfo) bool {
	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

// fileIdentity is what du asks of an entry, as busybox-w32's stat fills st_dev, st_ino and
// st_nlink: the volume's serial number, the file's index on it and its count of links. It
// costs a handle open and a call, one for all three. A link info has not followed is opened as
// itself.
func fileIdentity(path string, info os.FileInfo) (fileID, int, bool) {
	flags := uint32(0)
	if isLink(info) {
		flags = windows.FILE_FLAG_OPEN_REPARSE_POINT
	}
	handle, ok := openForMetadata(path, flags)
	if !ok {
		return fileID{}, 0, false
	}
	defer windows.CloseHandle(handle)
	var data windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &data); err != nil {
		return fileID{}, 0, false
	}
	index := uint64(data.FileIndexHigh)<<32 | uint64(data.FileIndexLow)
	return fileID{device: uint64(data.VolumeSerialNumber), index: index}, int(data.NumberOfLinks), true
}

// entrySize is an entry's length as busybox-w32's lstat gives it. A link's is the length of
// what it holds, which Windows keeps in the reparse point rather than in the file, so Lstat
// says 0.
func entrySize(path string, info os.FileInfo) int64 {
	if isLink(info) {
		if target, err := os.Readlink(path); err == nil {
			return int64(len(target))
		}
	}
	return info.Size()
}
