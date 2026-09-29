//go:build !windows && !linux

package applets

import "golang.org/x/sys/unix"

// statfsNameLength is the longest name the filesystem holds. The BSDs' and macOS's statfs does not
// say, and 255, their MAXNAMLEN, is what statvfs answers for APFS and HFS+ alike.
func statfsNameLength(*unix.Statfs_t) uint64 { return 255 }
