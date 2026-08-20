//go:build windows

package applets

import (
	"path/filepath"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// What `os.FileInfo` does not carry, and what `du` and `ls -l` need from Windows directly.
//
// No new dependency: `golang.org/x/sys/windows` is already linked, for `id` and for the
// console work in cmd/nemosh. What it costs is calls -- see each function.

// allocatedSize is how much room a file occupies on disk, in bytes.
//
// Not FILE_STANDARD_INFO's AllocationSize, which was the obvious answer and the wrong one:
// NTFS keeps a small file *inside* its MFT record and reports an allocation of zero for it, so
// a one-byte file came back as occupying nothing. busybox-w32 rounds the size up to the
// cluster instead and answers 4K, which is both what a user means by disk usage and what the
// primary reference prints. Measured: a 1-byte, a 5-byte and a 1500-byte file all cost 4K
// there, and this now agrees on all three.
//
// A directory costs nothing, which is also measured -- busybox says 0 for an empty one. Its
// entries live in the MFT record too.
//
// The cost is one GetDiskFreeSpace per volume, cached, and no per-file call at all. A sparse
// or compressed file is over-reported, which busybox does too.
func allocatedSize(path string, size int64, isDir bool) (int64, bool) {
	if isDir {
		return 0, true
	}
	cluster, ok := volumeClusterSize(path)
	if !ok {
		return 0, false
	}
	return (size + cluster - 1) / cluster * cluster, true
}

var procGetDiskFreeSpace = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetDiskFreeSpaceW")

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
	handle, ok := openForMetadata(path)
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
// still be asked; FILE_FLAG_BACKUP_SEMANTICS so a directory can be opened too.
func openForMetadata(path string) (windows.Handle, bool) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, false
	}
	handle, err := windows.CreateFile(name, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
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
