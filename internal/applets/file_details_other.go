//go:build !windows

package applets

import (
	"os"
	"os/user"
	"strconv"
	"syscall"
)

// Elsewhere both facts come from the stat the caller has already done, so there is no extra
// call and no platform rule to invent. See file_details_windows.go for the Windows side.

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

// fileOwnerName is the account that owns the file. os/user rather than a bare uid, because a
// number in that column is `ls -n` and this is `ls -l`; the pure-Go fallback reads
// /etc/passwd, so no cgo is pulled in.
func fileOwnerName(path string) string {
	info, err := os.Lstat(path)
	if err != nil {
		return "root"
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "root"
	}
	id := strconv.Itoa(int(stat.Uid))
	return cachedOwnerName(id, func() string {
		account, err := user.LookupId(id)
		if err != nil {
			return id
		}
		return account.Username
	})
}

// isSymbolicLink reports whether the entry is a link. Elsewhere the mode says so.
func isSymbolicLink(info os.FileInfo) bool {
	return info.Mode()&os.ModeSymlink != 0
}

// fileIdentity is st_dev, st_ino and st_nlink, from the stat du has already made.
func fileIdentity(_ string, info os.FileInfo) (fileID, int, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileID{}, 0, false
	}
	return fileID{device: uint64(stat.Dev), index: uint64(stat.Ino)}, int(stat.Nlink), true
}

// entrySize is st_size, which for a link is already the length of what it holds.
func entrySize(_ string, info os.FileInfo) int64 {
	return info.Size()
}

// allocatedBytes is st_blocks, which is in 512-byte units by definition whatever the
// filesystem's own block size happens to be, from the stat du has made, so a followed link's
// is its target's. A directory has blocks of its own here, unlike a small one on NTFS, and
// they are counted -- which is why `du` on an empty ext4 directory says 4.
func allocatedBytes(_ string, info os.FileInfo) (int64, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return stat.Blocks * 512, true
}
