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
	absolute, err := filepath.Abs(path)
	if err != nil {
		return 0, false
	}
	root := filepath.VolumeName(absolute) + string(filepath.Separator)
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
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, false
	}
	// No access rights beyond metadata, so a file another process holds open for writing
	// can still be asked; FILE_FLAG_BACKUP_SEMANTICS so a directory can be opened too.
	handle, err := windows.CreateFile(name, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
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
