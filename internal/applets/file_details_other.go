//go:build !windows

package applets

import (
	"os"
	"syscall"
)

// Elsewhere both facts come from the stat the caller has already done, so there is no extra
// call and no platform rule to invent. See file_details_windows.go for the Windows side.

// allocatedSize is st_blocks, which is in 512-byte units by definition whatever the
// filesystem's own block size happens to be. A directory has blocks of its own here, unlike
// on NTFS, and they are counted -- which is why GNU `du` on an empty ext4 directory says 4.
func allocatedSize(path string, _ int64, _ bool) (int64, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return stat.Blocks * 512, true
}

func fileLinkCount(path string) (int, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return int(stat.Nlink), true
}
